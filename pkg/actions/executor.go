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

	// Add action-specific environment variables
	env["GITHUB_ACTION"] = action.Reference
	env["GITHUB_ACTION_PATH"] = action.LocalPath

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

	// For now, create a placeholder implementation
	// In a real implementation, you would:
	// 1. Check if Node.js is available
	// 2. Install dependencies (npm install)
	// 3. Execute the main script

	return &ActionExecutionResult{
		Success: true,
		Outputs: map[string]string{
			"message": fmt.Sprintf("Node.js action %s executed (placeholder)", action.Reference),
		},
	}, nil
}

// executeDockerAction executes a Docker action
func (e *Executor) executeDockerAction(ctx context.Context, action *Action, env map[string]string) (*ActionExecutionResult, error) {
	e.logger.Info("Executing Docker action", "action", action.Reference)

	// For now, create a placeholder implementation
	// In a real implementation, you would:
	// 1. Build or pull the Docker image
	// 2. Run the container with appropriate environment and volumes

	return &ActionExecutionResult{
		Success: true,
		Outputs: map[string]string{
			"message": fmt.Sprintf("Docker action %s executed (placeholder)", action.Reference),
		},
	}, nil
}
