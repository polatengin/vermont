package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/polatengin/vermont/internal/logger"
	"gopkg.in/yaml.v3"
)

// Executor handles action execution
type Executor struct {
	manager *Manager
	logger  *logger.Logger
}

// NewExecutor creates a new action executor
func NewExecutor(manager *Manager, logger *logger.Logger) *Executor {
	return &Executor{
		manager: manager,
		logger:  logger,
	}
}

// Execute executes an action with the given inputs
func (e *Executor) Execute(ctx context.Context, reference string, inputs map[string]interface{}, env map[string]string) (*ActionExecutionResult, error) {
	// Get the action
	action, err := e.manager.GetAction(ctx, reference)
	if err != nil {
		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to get action: %v", err),
		}, nil
	}

	// Load action metadata
	metadata, err := e.loadActionMetadata(action.LocalPath)
	if err != nil {
		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to load action metadata: %v", err),
		}, nil
	}

	action.Metadata = metadata

	// Execute based on action type
	return e.executeAction(ctx, action, inputs, env)
}

// loadActionMetadata loads action.yml or action.yaml metadata
func (e *Executor) loadActionMetadata(actionPath string) (*ActionMetadata, error) {
	// Try action.yml first, then action.yaml
	var metadataFile string
	actionYml := filepath.Join(actionPath, "action.yml")
	actionYaml := filepath.Join(actionPath, "action.yaml")

	if _, err := os.Stat(actionYml); err == nil {
		metadataFile = actionYml
	} else if _, err := os.Stat(actionYaml); err == nil {
		metadataFile = actionYaml
	} else {
		return nil, fmt.Errorf("action.yml or action.yaml not found in %s", actionPath)
	}

	data, err := os.ReadFile(metadataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	var metadata ActionMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata: %w", err)
	}

	return &metadata, nil
}

// executeAction executes an action based on its type
func (e *Executor) executeAction(ctx context.Context, action *Action, inputs map[string]interface{}, env map[string]string) (*ActionExecutionResult, error) {
	metadata := action.Metadata
	if metadata == nil {
		return &ActionExecutionResult{
			Success: false,
			Error:   "action metadata not loaded",
		}, nil
	}

	// Validate required inputs
	if err := e.validateInputs(metadata.Inputs, inputs); err != nil {
		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("input validation failed: %v", err),
		}, nil
	}

	// Set up action environment
	actionEnv := e.createActionEnvironment(action, inputs, env)

	switch strings.ToLower(metadata.Runs.Using) {
	case "composite":
		return e.executeCompositeAction(ctx, action, actionEnv)
	case "node12", "node16", "node20":
		return e.executeNodeAction(ctx, action, actionEnv)
	case "docker":
		return e.executeDockerAction(ctx, action, actionEnv)
	default:
		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported action runtime: %s", metadata.Runs.Using),
		}, nil
	}
}

// validateInputs validates that all required inputs are provided
func (e *Executor) validateInputs(inputDefs map[string]ActionInput, inputs map[string]interface{}) error {
	for name, def := range inputDefs {
		if def.Required {
			if _, exists := inputs[name]; !exists {
				// Check if there's a default value
				if def.Default == "" {
					return fmt.Errorf("required input '%s' not provided", name)
				}
			}
		}
	}
	return nil
}

// createActionEnvironment creates environment variables for action execution
func (e *Executor) createActionEnvironment(action *Action, inputs map[string]interface{}, baseEnv map[string]string) map[string]string {
	env := make(map[string]string)

	// Copy base environment
	for k, v := range baseEnv {
		env[k] = v
	}

	// Add action inputs as environment variables (INPUT_{NAME})
	for name, value := range inputs {
		envName := fmt.Sprintf("INPUT_%s", strings.ToUpper(strings.ReplaceAll(name, "-", "_")))
		env[envName] = fmt.Sprintf("%v", value)
	}

	// Special handling for common actions that need default inputs
	// If this is actions/checkout and no token is provided, use GITHUB_TOKEN if available
	if strings.Contains(action.Reference, "actions/checkout") {
		if _, tokenProvided := inputs["token"]; !tokenProvided {
			if githubToken := env["GITHUB_TOKEN"]; githubToken != "" {
				env["INPUT_TOKEN"] = githubToken
			}
		}
	}

	// Add action-specific environment variables
	env["GITHUB_ACTION"] = action.Reference
	env["GITHUB_ACTION_PATH"] = action.LocalPath

	// Add standard GitHub Actions environment variables if not already present
	if env["GITHUB_WORKSPACE"] == "" {
		if workDir := env["_VERMONT_WORK_DIR"]; workDir != "" {
			env["GITHUB_WORKSPACE"] = workDir
		} else {
			env["GITHUB_WORKSPACE"] = "/workspace"
		}
	}

	// Set default values for common GitHub Actions environment variables
	if env["GITHUB_REPOSITORY"] == "" {
		env["GITHUB_REPOSITORY"] = "owner/repo" // Default placeholder
	}
	if env["GITHUB_REF"] == "" {
		env["GITHUB_REF"] = "refs/heads/main"
	}
	if env["GITHUB_SHA"] == "" {
		env["GITHUB_SHA"] = "0000000000000000000000000000000000000000"
	}
	if env["GITHUB_EVENT_NAME"] == "" {
		env["GITHUB_EVENT_NAME"] = "workflow_dispatch"
	}
	if env["RUNNER_OS"] == "" {
		env["RUNNER_OS"] = "Linux"
	}
	if env["RUNNER_ARCH"] == "" {
		env["RUNNER_ARCH"] = "X64"
	}

	// Add metadata environment variables if available
	if action.Metadata != nil {
		for k, v := range action.Metadata.Runs.Env {
			env[k] = v
		}
	}

	return env
}

// executeCompositeAction executes a composite action
func (e *Executor) executeCompositeAction(ctx context.Context, action *Action, env map[string]string) (*ActionExecutionResult, error) {
	e.logger.Info("Executing composite action", "action", action.Reference)

	if len(action.Metadata.Runs.Steps) == 0 {
		return &ActionExecutionResult{
			Success: false,
			Error:   "composite action has no steps defined",
		}, nil
	}

	// Create template processor
	inputs := make(map[string]interface{})
	for k, v := range env {
		if strings.HasPrefix(k, "INPUT_") {
			inputName := strings.ToLower(strings.ReplaceAll(k[6:], "_", "-"))
			inputs[inputName] = v
		}
	}

	templateProcessor := NewTemplateProcessor(inputs, env)
	outputs := make(map[string]string)

	// Create temporary directory for GITHUB_OUTPUT
	tmpDir := "/tmp/vermont-action-" + strings.ReplaceAll(action.Reference, "/", "-")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	githubOutputFile := filepath.Join(tmpDir, "github_output")

	// Add GITHUB_OUTPUT to environment
	actionEnv := make(map[string]string)
	for k, v := range env {
		actionEnv[k] = v
	}
	actionEnv["GITHUB_OUTPUT"] = githubOutputFile

	// Execute each step
	for i, step := range action.Metadata.Runs.Steps {
		stepResult, err := e.executeCompositeStep(ctx, action, step, actionEnv, templateProcessor)
		if err != nil {
			return &ActionExecutionResult{
				Success: false,
				Error:   fmt.Sprintf("step %d failed: %v", i+1, err),
			}, nil
		}

		if !stepResult.Success {
			return &ActionExecutionResult{
				Success: false,
				Error:   fmt.Sprintf("step %d failed: %s", i+1, stepResult.Error),
			}, nil
		}

		// Merge outputs
		for k, v := range stepResult.Outputs {
			outputs[k] = v
		}
	}

	// Read outputs from GITHUB_OUTPUT file
	if data, err := os.ReadFile(githubOutputFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					outputs[parts[0]] = parts[1]
				}
			}
		}
	}

	// Clean up
	if err := os.RemoveAll(tmpDir); err != nil {
		e.logger.Warn("Failed to remove temporary directory", "path", tmpDir, "error", err)
	}

	return &ActionExecutionResult{
		Success: true,
		Outputs: outputs,
	}, nil
}

// executeCompositeStep executes a single step in a composite action
func (e *Executor) executeCompositeStep(ctx context.Context, action *Action, step ActionRunStep, env map[string]string, templateProcessor *TemplateProcessor) (*ActionExecutionResult, error) {
	// Create step environment
	stepEnv := make(map[string]string)
	for k, v := range env {
		stepEnv[k] = v
	}
	for k, v := range step.Env {
		stepEnv[k] = templateProcessor.ProcessTemplate(v)
	}

	if step.Run != "" {
		// Process template variables in the run command
		processedCommand := templateProcessor.ProcessTemplate(step.Run)

		// Execute shell command
		return e.executeShellStep(ctx, action.LocalPath, processedCommand, step.Shell, stepEnv)
	} else if step.Uses != "" {
		// Execute nested action
		inputs := make(map[string]interface{})
		for k, v := range step.With {
			inputs[k] = templateProcessor.ProcessTemplate(v)
		}
		return e.Execute(ctx, step.Uses, inputs, stepEnv)
	}

	return &ActionExecutionResult{
		Success: true,
	}, nil
}

// executeShellStep executes a shell command step
func (e *Executor) executeShellStep(ctx context.Context, workingDir, command, shell string, env map[string]string) (*ActionExecutionResult, error) {
	start := time.Now()

	// Use bash as default shell
	if shell == "" {
		shell = "bash"
	}

	e.logger.Info("Executing shell step",
		"command", command,
		"shell", shell,
		"workingDir", workingDir)

	// Execute the command
	cmd := exec.CommandContext(ctx, shell, "-c", command)

	// Set working directory
	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Set environment variables
	cmd.Env = os.Environ()
	for key, value := range env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	// Execute command and capture output
	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	if err != nil {
		e.logger.Error("Shell command failed",
			"command", command,
			"error", err,
			"output", string(output))

		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Command failed: %v - Output: %s", err, string(output)),
		}, nil
	}

	e.logger.Debug("Shell command completed",
		"command", command,
		"duration", duration,
		"output", string(output))

	// Parse outputs from command output if needed
	// For now, just return the output as a message
	outputs := map[string]string{}
	if strings.TrimSpace(string(output)) != "" {
		outputs["message"] = strings.TrimSpace(string(output))
	}

	return &ActionExecutionResult{
		Success:  true,
		Outputs:  outputs,
		Duration: duration.String(),
	}, nil
}

// executeNodeAction executes a Node.js action
func (e *Executor) executeNodeAction(ctx context.Context, action *Action, env map[string]string) (*ActionExecutionResult, error) {
	e.logger.Info("Executing Node.js action", "action", action.Reference)

	// Check if we're in container mode
	containerMode := env["_VERMONT_CONTAINER_MODE"] == "true"
	containerImage := env["_VERMONT_CONTAINER_IMAGE"]
	workDir := env["_VERMONT_WORK_DIR"]

	if containerMode && containerImage != "" {
		return e.executeNodeActionInContainer(ctx, action, env, containerImage, workDir)
	}

	// Check if Node.js is available on host
	if _, err := exec.LookPath("node"); err != nil {
		e.logger.Warn("Node.js not found, action may fail", "action", action.Reference)
		return &ActionExecutionResult{
			Success: false,
			Error:   "Node.js runtime not available. Please install Node.js or use a container with Node.js",
		}, nil
	}

	return e.executeNodeActionOnHost(ctx, action, env)
}

// executeNodeActionOnHost executes a Node.js action on the host system
func (e *Executor) executeNodeActionOnHost(ctx context.Context, action *Action, env map[string]string) (*ActionExecutionResult, error) {

	// Get the main script from metadata
	mainScript := action.Metadata.Runs.Main
	if mainScript == "" {
		return &ActionExecutionResult{
			Success: false,
			Error:   "Node.js action missing 'main' script in runs configuration",
		}, nil
	}

	// Full path to the main script
	scriptPath := filepath.Join(action.LocalPath, mainScript)
	if _, err := os.Stat(scriptPath); err != nil {
		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Main script not found: %s", scriptPath),
		}, nil
	}

	// Check if package.json exists and install dependencies if needed
	packageJsonPath := filepath.Join(action.LocalPath, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		// Check if node_modules exists
		nodeModulesPath := filepath.Join(action.LocalPath, "node_modules")
		if _, err := os.Stat(nodeModulesPath); err != nil {
			e.logger.Info("Installing Node.js dependencies", "action", action.Reference)

			// Run npm install
			cmd := exec.CommandContext(ctx, "npm", "install", "--production")
			cmd.Dir = action.LocalPath
			cmd.Env = os.Environ()

			if output, err := cmd.CombinedOutput(); err != nil {
				e.logger.Warn("npm install failed", "action", action.Reference, "output", string(output))
				// Continue anyway, some actions might work without all dependencies
			}
		}
	}

	// Create temporary directory for outputs
	tmpDir := "/tmp/vermont-action-" + strings.ReplaceAll(action.Reference, "/", "-")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}

	// Set up GitHub Actions environment variables
	actionEnv := make([]string, 0, len(env)+10)
	for k, v := range env {
		actionEnv = append(actionEnv, fmt.Sprintf("%s=%s", k, v))
	}

	// Add GitHub Actions specific environment variables
	githubOutputFile := filepath.Join(tmpDir, "github_output")
	githubStepSummaryFile := filepath.Join(tmpDir, "github_step_summary")

	actionEnv = append(actionEnv, fmt.Sprintf("GITHUB_OUTPUT=%s", githubOutputFile))
	actionEnv = append(actionEnv, fmt.Sprintf("GITHUB_STEP_SUMMARY=%s", githubStepSummaryFile))
	actionEnv = append(actionEnv, fmt.Sprintf("RUNNER_TEMP=%s", tmpDir))
	actionEnv = append(actionEnv, fmt.Sprintf("RUNNER_TOOL_CACHE=%s/tool-cache", tmpDir))

	// Execute the Node.js script
	e.logger.Debug("Running Node.js action", "script", scriptPath)

	cmd := exec.CommandContext(ctx, "node", scriptPath)
	cmd.Dir = action.LocalPath
	cmd.Env = actionEnv

	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Read outputs from GITHUB_OUTPUT file
	outputs := make(map[string]string)
	if data, err := os.ReadFile(githubOutputFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					outputs[parts[0]] = parts[1]
				}
			}
		}
	}

	// Clean up temporary directory
	defer func() {
		if cleanupErr := os.RemoveAll(tmpDir); cleanupErr != nil {
			e.logger.Warn("Failed to clean up temporary directory", "dir", tmpDir, "error", cleanupErr)
		}
	}()

	if err != nil {
		e.logger.Error("Node.js action execution failed",
			"action", action.Reference,
			"error", err,
			"output", outputStr)

		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Node.js action failed: %v\nOutput: %s", err, outputStr),
			Outputs: outputs,
		}, nil
	}

	e.logger.Info("Node.js action completed successfully", "action", action.Reference)

	return &ActionExecutionResult{
		Success: true,
		Outputs: outputs,
	}, nil
}

// executeDockerAction executes a Docker action
func (e *Executor) executeDockerAction(ctx context.Context, action *Action, env map[string]string) (*ActionExecutionResult, error) {
	e.logger.Info("Executing Docker action", "action", action.Reference)

	// Check if Docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return &ActionExecutionResult{
			Success: false,
			Error:   "Docker runtime not available. Please install Docker or use host execution",
		}, nil
	}

	// Get Docker configuration from metadata
	image := action.Metadata.Runs.Image
	if image == "" {
		return &ActionExecutionResult{
			Success: false,
			Error:   "Docker action missing 'image' in runs configuration",
		}, nil
	}

	// Handle different image types
	var dockerImage string
	if strings.HasPrefix(image, "docker://") {
		// Registry image
		dockerImage = strings.TrimPrefix(image, "docker://")
	} else if strings.HasPrefix(image, "./") || !strings.Contains(image, ":") {
		// Local Dockerfile - build it
		dockerfileDir := action.LocalPath
		if strings.HasPrefix(image, "./") {
			dockerfileDir = filepath.Join(action.LocalPath, strings.TrimPrefix(image, "./"))
		}

		// Build the Docker image
		imageName := fmt.Sprintf("vermont-action-%s:%s",
			strings.ReplaceAll(action.Owner, "/", "-"),
			strings.ReplaceAll(action.Name, "/", "-"))

		e.logger.Info("Building Docker image for action", "action", action.Reference, "image", imageName)

		buildCmd := exec.CommandContext(ctx, "docker", "build", "-t", imageName, dockerfileDir)
		if output, err := buildCmd.CombinedOutput(); err != nil {
			return &ActionExecutionResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to build Docker image: %v\nOutput: %s", err, string(output)),
			}, nil
		}

		dockerImage = imageName
	} else {
		// Assume it's a registry image
		dockerImage = image
	}

	// Pull the image if it's from a registry
	if strings.Contains(dockerImage, "/") && !strings.HasPrefix(dockerImage, "vermont-action-") {
		e.logger.Info("Pulling Docker image", "image", dockerImage)
		pullCmd := exec.CommandContext(ctx, "docker", "pull", dockerImage)
		if output, err := pullCmd.CombinedOutput(); err != nil {
			e.logger.Warn("Failed to pull Docker image", "image", dockerImage, "error", err, "output", string(output))
			// Continue anyway, image might already exist locally
		}
	}

	// Create temporary directory for outputs
	tmpDir := "/tmp/vermont-action-" + strings.ReplaceAll(action.Reference, "/", "-")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tmpDir); cleanupErr != nil {
			e.logger.Warn("Failed to clean up temporary directory", "dir", tmpDir, "error", cleanupErr)
		}
	}()

	// Set up GitHub Actions environment variables
	githubOutputFile := filepath.Join(tmpDir, "github_output")
	_ = filepath.Join(tmpDir, "github_step_summary") // For future use

	// Build docker run command
	dockerArgs := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/github/workspace", e.manager.config.Runner.WorkDir),
		"-v", fmt.Sprintf("%s:/tmp/runner", tmpDir),
		"-e", "GITHUB_OUTPUT=/tmp/runner/github_output",
		"-e", "GITHUB_STEP_SUMMARY=/tmp/runner/github_step_summary",
		"-e", "RUNNER_TEMP=/tmp/runner",
		"-e", "RUNNER_TOOL_CACHE=/tmp/runner/tool-cache",
		"--workdir", "/github/workspace",
	}

	// Add environment variables
	for k, v := range env {
		dockerArgs = append(dockerArgs, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Add the image
	dockerArgs = append(dockerArgs, dockerImage)

	// Add entrypoint and args if specified
	if action.Metadata.Runs.Entrypoint != "" {
		dockerArgs = append(dockerArgs, "--entrypoint", action.Metadata.Runs.Entrypoint)
	}

	// Add arguments
	dockerArgs = append(dockerArgs, action.Metadata.Runs.Args...)

	// Execute the Docker container
	e.logger.Debug("Running Docker action", "image", dockerImage, "args", dockerArgs)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Read outputs from GITHUB_OUTPUT file
	outputs := make(map[string]string)
	if data, err := os.ReadFile(githubOutputFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					outputs[parts[0]] = parts[1]
				}
			}
		}
	}

	if err != nil {
		e.logger.Error("Docker action execution failed",
			"action", action.Reference,
			"image", dockerImage,
			"error", err,
			"output", outputStr)

		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Docker action failed: %v\nOutput: %s", err, outputStr),
			Outputs: outputs,
		}, nil
	}

	e.logger.Info("Docker action completed successfully", "action", action.Reference)

	return &ActionExecutionResult{
		Success: true,
		Outputs: outputs,
	}, nil
}

// executeNodeActionInContainer executes a Node.js action inside a container
func (e *Executor) executeNodeActionInContainer(ctx context.Context, action *Action, env map[string]string, containerImage, workDir string) (*ActionExecutionResult, error) {
	e.logger.Info("Executing Node.js action in container", "action", action.Reference, "image", containerImage)

	// Get the main script from metadata
	mainScript := action.Metadata.Runs.Main
	if mainScript == "" {
		return &ActionExecutionResult{
			Success: false,
			Error:   "Node.js action missing 'main' script in runs configuration",
		}, nil
	}

	// Create temporary directory for outputs
	tmpDir := fmt.Sprintf("/tmp/vermont-action-%d", time.Now().UnixNano())
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer func() {
		if cleanupErr := os.RemoveAll(tmpDir); cleanupErr != nil {
			e.logger.Warn("Failed to clean up temporary directory", "dir", tmpDir, "error", cleanupErr)
		}
	}()

	// Create container ID
	containerID := fmt.Sprintf("vermont-node-action-%d", time.Now().UnixNano())

	// Build docker run command
	args := []string{
		"run", "--rm",
		"--name", containerID,
		"--workdir", "/workspace",
		"-v", fmt.Sprintf("%s:/workspace", workDir),
		"-v", fmt.Sprintf("%s:/tmp/runner", tmpDir),
		"-v", fmt.Sprintf("%s:/action-source:ro", action.LocalPath),
		"-e", "GITHUB_OUTPUT=/tmp/runner/github_output",
		"-e", "GITHUB_STEP_SUMMARY=/tmp/runner/github_step_summary",
		"-e", "RUNNER_TEMP=/tmp/runner",
		"-e", fmt.Sprintf("RUNNER_TOOL_CACHE=%s/tool-cache", tmpDir),
	}

	// Add environment variables (exclude Vermont-specific ones)
	for k, v := range env {
		if !strings.HasPrefix(k, "_VERMONT_") {
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}

	// Add standard GitHub Actions environment variables that actions expect
	args = append(args, "-e", "GITHUB_WORKSPACE=/workspace")
	args = append(args, "-e", "GITHUB_ACTION_PATH=/tmp/action")
	args = append(args, "-e", fmt.Sprintf("GITHUB_ACTION=%s", action.Reference))
	args = append(args, "-e", "RUNNER_OS=Linux")
	args = append(args, "-e", "RUNNER_ARCH=X64")

	// Add the image
	args = append(args, containerImage)

	// Build the command to copy action and run it
	nodeCmd := fmt.Sprintf(`
		# Copy action to writable location
		cp -r /action-source /tmp/action
		cd /tmp/action
		
		# Install dependencies if package.json exists
		if [ -f package.json ] && [ ! -d node_modules ]; then
			echo "Installing Node.js dependencies..."
			npm install --production
		fi
		
		# Run the action from workspace directory
		cd /workspace
		node /tmp/action/%s
	`, mainScript)

	args = append(args, "bash", "-c", nodeCmd)

	e.logger.Debug("Running Node.js action in container", "action", action.Reference, "command", nodeCmd)

	// Execute the container
	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	// Read outputs from GITHUB_OUTPUT file
	outputs := make(map[string]string)
	githubOutputFile := filepath.Join(tmpDir, "github_output")
	if data, err := os.ReadFile(githubOutputFile); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line != "" && strings.Contains(line, "=") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					outputs[parts[0]] = parts[1]
				}
			}
		}
	}

	if err != nil {
		e.logger.Error("Container Node.js action execution failed",
			"action", action.Reference,
			"image", containerImage,
			"error", err,
			"output", outputStr)

		return &ActionExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Container Node.js action failed: %v\nOutput: %s", err, outputStr),
			Outputs: outputs,
		}, nil
	}

	e.logger.Info("Container Node.js action completed successfully", "action", action.Reference)

	return &ActionExecutionResult{
		Success: true,
		Outputs: outputs,
	}, nil
}
