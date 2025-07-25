package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Env map[string]string `json:"env"`
}

// Workflow represents a GitHub Actions workflow
type Workflow struct {
	Name string          `yaml:"name"`
	On   interface{}     `yaml:"on"`
	Jobs map[string]*Job `yaml:"jobs"`
}

// Job represents a single job in a workflow
type Job struct {
	RunsOn   interface{} `yaml:"runs-on"`
	Needs    []string    `yaml:"needs"`
	Steps    []*Step     `yaml:"steps"`
	Strategy *Strategy   `yaml:"strategy"`
}

// Strategy represents the strategy configuration for a job
type Strategy struct {
	Matrix map[string]interface{} `yaml:"matrix"`
}

// Step represents a single step in a job
type Step struct {
	Name string                 `yaml:"name"`
	Run  string                 `yaml:"run"`
	Uses string                 `yaml:"uses"`
	With map[string]interface{} `yaml:"with"`
	Env  map[string]string      `yaml:"env"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: vermont <workflow-file>")
		fmt.Println("Example: vermont examples/parallel-test.yml")
		os.Exit(1)
	}

	workflowFile := os.Args[1]

	// Load configuration
	config, err := loadConfig("config.json")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Load workflow
	workflow, err := loadWorkflow(workflowFile)
	if err != nil {
		log.Fatalf("Failed to load workflow: %v", err)
	}

	// Execute workflow
	if err := executeWorkflow(workflow, config); err != nil {
		log.Fatalf("Failed to execute workflow: %v", err)
	}

	fmt.Println("Workflow completed successfully!")
}

func loadConfig(configFile string) (*Config, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Expand environment variables
	for key, value := range config.Env {
		if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
			envVar := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
			if envValue := os.Getenv(envVar); envValue != "" {
				config.Env[key] = envValue
			} else {
				config.Env[key] = "fake-" + strings.ToLower(envVar)
			}
		}
	}

	return &config, nil
}

// expandEnvironmentVariables expands ${VAR} syntax in strings using shell environment
func expandEnvironmentVariables(value string) string {
	// Handle ${VAR} syntax
	if strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}") {
		envVar := strings.TrimSuffix(strings.TrimPrefix(value, "${"), "}")
		if envValue := os.Getenv(envVar); envValue != "" {
			return envValue
		}
		// Return original value if environment variable not found
		return value
	}
	return value
}

func loadWorkflow(workflowFile string) (*Workflow, error) {
	data, err := os.ReadFile(workflowFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	return &workflow, nil
}

// expandMatrixJobs takes jobs with matrix strategies and expands them into multiple jobs
func expandMatrixJobs(jobs map[string]*Job) map[string]*Job {
	expandedJobs := make(map[string]*Job)

	for jobName, job := range jobs {
		if job.Strategy != nil && job.Strategy.Matrix != nil {
			// Generate all matrix combinations
			combinations := generateMatrixCombinations(job.Strategy.Matrix)

			for i, combination := range combinations {
				// Create unique job name for each matrix combination
				matrixJobName := fmt.Sprintf("%s_%d", jobName, i)

				// Clone the job
				matrixJob := &Job{
					RunsOn: job.RunsOn,
					Needs:  job.Needs,
					Steps:  cloneSteps(job.Steps, combination),
				}

				expandedJobs[matrixJobName] = matrixJob
			}
		} else {
			// Job without matrix, just copy as-is
			expandedJobs[jobName] = job
		}
	}

	return expandedJobs
}

// generateMatrixCombinations generates all possible combinations from a matrix
func generateMatrixCombinations(matrix map[string]interface{}) []map[string]interface{} {
	var combinations []map[string]interface{}

	// Separate matrix dimensions from include/exclude directives
	dimensions := make(map[string]interface{})
	var includeList []map[string]interface{}
	var excludeList []map[string]interface{}

	for key, value := range matrix {
		switch key {
		case "include":
			if includes, ok := value.([]interface{}); ok {
				for _, include := range includes {
					if includeMap, ok := include.(map[string]interface{}); ok {
						includeList = append(includeList, includeMap)
					}
				}
			}
		case "exclude":
			if excludes, ok := value.([]interface{}); ok {
				for _, exclude := range excludes {
					if excludeMap, ok := exclude.(map[string]interface{}); ok {
						excludeList = append(excludeList, excludeMap)
					}
				}
			}
		default:
			dimensions[key] = value
		}
	}

	// Get all keys and values for base dimensions
	keys := make([]string, 0, len(dimensions))
	values := make([][]interface{}, 0, len(dimensions))

	for key, value := range dimensions {
		keys = append(keys, key)
		switch v := value.(type) {
		case []interface{}:
			values = append(values, v)
		default:
			values = append(values, []interface{}{v})
		}
	}

	// Generate cartesian product for base dimensions
	var generate func(int, map[string]interface{})
	generate = func(index int, current map[string]interface{}) {
		if index == len(keys) {
			// Make a copy of current combination
			combination := make(map[string]interface{})
			for k, v := range current {
				combination[k] = v
			}

			// Check if this combination should be excluded
			excluded := false
			for _, exclude := range excludeList {
				if matchesCombination(combination, exclude) {
					excluded = true
					break
				}
			}

			if !excluded {
				combinations = append(combinations, combination)
			}
			return
		}

		key := keys[index]
		for _, value := range values[index] {
			current[key] = value
			generate(index+1, current)
		}
	}

	generate(0, make(map[string]interface{}))

	// Add include combinations
	for _, include := range includeList {
		// Check if any existing combination matches the base dimensions of include
		matchFound := false
		for i, existing := range combinations {
			if matchesBaseDimensions(existing, include, keys) {
				// Merge additional properties from include
				for k, v := range include {
					if !contains(keys, k) {
						combinations[i][k] = v
					}
				}
				matchFound = true
				break
			}
		}

		// If no match found, add as new combination
		if !matchFound {
			combinations = append(combinations, include)
		}
	}

	return combinations
}

// matchesCombination checks if a combination matches all fields in a pattern
func matchesCombination(combination, pattern map[string]interface{}) bool {
	for key, value := range pattern {
		if combination[key] != value {
			return false
		}
	}
	return true
}

// matchesBaseDimensions checks if a combination matches the base dimensions of an include
func matchesBaseDimensions(existing, include map[string]interface{}, baseDimensions []string) bool {
	for _, dim := range baseDimensions {
		if includeVal, exists := include[dim]; exists {
			if existing[dim] != includeVal {
				return false
			}
		}
	}
	return true
}

// contains checks if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// cloneSteps clones steps and substitutes matrix variables
func cloneSteps(steps []*Step, matrixVars map[string]interface{}) []*Step {
	clonedSteps := make([]*Step, len(steps))

	for i, step := range steps {
		clonedSteps[i] = &Step{
			Name: substituteMatrixVars(step.Name, matrixVars),
			Run:  substituteMatrixVars(step.Run, matrixVars),
			Uses: substituteMatrixVars(step.Uses, matrixVars),
			With: cloneWithVars(step.With, matrixVars),
			Env:  cloneEnvVars(step.Env, matrixVars),
		}
	}

	return clonedSteps
}

// substituteMatrixVars replaces ${{ matrix.* }} variables in strings
func substituteMatrixVars(text string, matrixVars map[string]interface{}) string {
	result := text
	
	// First, substitute all variables that exist in matrixVars
	for key, value := range matrixVars {
		placeholder := fmt.Sprintf("${{ matrix.%s }}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	
	// Then, find any remaining matrix variables and substitute them with empty string
	// This handles cases where a matrix variable is referenced but doesn't exist in this combination
	for {
		start := strings.Index(result, "${{ matrix.")
		if start == -1 {
			break
		}
		end := strings.Index(result[start:], " }}")
		if end == -1 {
			break
		}
		end += start + 3 // Include the " }}"
		
		// Replace unknown matrix variable with empty string
		result = result[:start] + result[end:]
	}
	
	return result
}

// cloneWithVars clones and substitutes matrix vars in with map
func cloneWithVars(with map[string]interface{}, matrixVars map[string]interface{}) map[string]interface{} {
	if with == nil {
		return nil
	}

	cloned := make(map[string]interface{})
	for key, value := range with {
		if strValue, ok := value.(string); ok {
			cloned[key] = substituteMatrixVars(strValue, matrixVars)
		} else {
			cloned[key] = value
		}
	}
	return cloned
}

// cloneEnvVars clones and substitutes matrix vars in env map
func cloneEnvVars(env map[string]string, matrixVars map[string]interface{}) map[string]string {
	if env == nil {
		return nil
	}

	cloned := make(map[string]string)
	for key, value := range env {
		cloned[key] = substituteMatrixVars(value, matrixVars)
	}
	return cloned
}

// substituteActionTemplates replaces action template variables in strings
func substituteActionTemplates(text string, inputs map[string]interface{}, stepOutputs map[string]map[string]string) string {
	result := text

	// Substitute inputs: ${{ inputs.name }}
	for inputName, value := range inputs {
		placeholder := fmt.Sprintf("${{ inputs.%s }}", inputName)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}

	// Substitute step outputs: ${{ steps.stepid.outputs.outputname }}
	for stepId, outputs := range stepOutputs {
		for outputName, value := range outputs {
			placeholder := fmt.Sprintf("${{ steps.%s.outputs.%s }}", stepId, outputName)
			result = strings.ReplaceAll(result, placeholder, value)
		}
	}

	return result
}

// parseStepOutputs reads outputs from GITHUB_OUTPUT file
func parseStepOutputs(githubOutputPath string) (map[string]string, error) {
	outputs := make(map[string]string)

	if _, err := os.Stat(githubOutputPath); os.IsNotExist(err) {
		return outputs, nil
	}

	data, err := os.ReadFile(githubOutputPath)
	if err != nil {
		return outputs, err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse name=value format
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			outputs[parts[0]] = parts[1]
		}
	}

	return outputs, nil
}

// ActionRef represents a parsed action reference
type ActionRef struct {
	Owner     string
	Repo      string
	Ref       string // version, branch, or commit
	IsLocal   bool
	LocalPath string
}

// parseActionRef parses action reference like "actions/checkout@v4" or "./path/to/action"
func parseActionRef(uses string) (*ActionRef, error) {
	// Handle relative paths (./path/to/action)
	if strings.HasPrefix(uses, "./") {
		return &ActionRef{
			IsLocal:   true,
			LocalPath: uses,
		}, nil
	}

	// Split by @ to get ref
	parts := strings.Split(uses, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid action reference format: %s (expected owner/repo@ref)", uses)
	}

	// Split owner/repo
	ownerRepo := strings.Split(parts[0], "/")
	if len(ownerRepo) != 2 {
		return nil, fmt.Errorf("invalid action reference format: %s (expected owner/repo@ref)", uses)
	}

	return &ActionRef{
		Owner: ownerRepo[0],
		Repo:  ownerRepo[1],
		Ref:   parts[1],
	}, nil
}

// cloneAction clones an action repository to the steps directory or resolves local path
func cloneAction(actionRef *ActionRef, stepsDir string) (string, error) {
	// Handle local actions
	if actionRef.IsLocal {
		// Get absolute path relative to current working directory
		currentDir, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get current directory: %w", err)
		}
		actionDir := filepath.Join(currentDir, actionRef.LocalPath)

		// Check if local action exists
		if _, err := os.Stat(actionDir); err != nil {
			return "", fmt.Errorf("local action not found: %s", actionDir)
		}

		fmt.Printf("      Using local action: %s\n", actionDir)
		return actionDir, nil
	}

	// Handle remote actions
	actionDir := filepath.Join(stepsDir, fmt.Sprintf("%s_%s_%s", actionRef.Owner, actionRef.Repo, actionRef.Ref))

	// Check if already cloned
	if _, err := os.Stat(actionDir); err == nil {
		return actionDir, nil
	}

	// Clone repository
	repoURL := fmt.Sprintf("https://github.com/%s/%s.git", actionRef.Owner, actionRef.Repo)
	fmt.Printf("      Cloning action: %s@%s\n", repoURL, actionRef.Ref)

	// Clone with specific ref
	cmd := exec.Command("git", "clone", "--depth", "1", "--branch", actionRef.Ref, repoURL, actionDir)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// If branch clone fails, try cloning and checking out the ref
		fmt.Printf("      Branch clone failed, trying full clone and checkout...\n")

		// Remove failed directory
		if removeErr := os.RemoveAll(actionDir); removeErr != nil {
			fmt.Printf("      Warning: failed to remove failed directory: %v\n", removeErr)
		}

		// Full clone
		cmd = exec.Command("git", "clone", repoURL, actionDir)
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to clone action repository: %w", err)
		}

		// Checkout specific ref
		cmd = exec.Command("git", "checkout", actionRef.Ref)
		cmd.Dir = actionDir
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to checkout ref %s: %w", actionRef.Ref, err)
		}
	}

	return actionDir, nil
}

// executeAction executes a GitHub Action
func executeAction(step *Step, jobDir, runnerImage string, config *Config, stepsDir string) error {
	// Parse action reference
	actionRef, err := parseActionRef(step.Uses)
	if err != nil {
		return fmt.Errorf("failed to parse action reference: %w", err)
	}

	// Clone action
	actionDir, err := cloneAction(actionRef, stepsDir)
	if err != nil {
		return fmt.Errorf("failed to clone action: %w", err)
	}

	// Read action.yml or action.yaml
	actionFile := ""
	for _, filename := range []string{"action.yml", "action.yaml"} {
		path := filepath.Join(actionDir, filename)
		if _, err := os.Stat(path); err == nil {
			actionFile = path
			break
		}
	}

	if actionFile == "" {
		return fmt.Errorf("action.yml or action.yaml not found in action directory")
	}

	// Parse action metadata
	actionData, err := os.ReadFile(actionFile)
	if err != nil {
		return fmt.Errorf("failed to read action file: %w", err)
	}

	var actionMeta struct {
		Runs struct {
			Using string `yaml:"using"`
			Main  string `yaml:"main"`
			Steps []struct {
				Name string                 `yaml:"name"`
				Run  string                 `yaml:"run"`
				Uses string                 `yaml:"uses"`
				With map[string]interface{} `yaml:"with"`
				Env  map[string]string      `yaml:"env"`
				ID   string                 `yaml:"id"`
			} `yaml:"steps"`
		} `yaml:"runs"`
		Inputs map[string]struct {
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
			Default     string `yaml:"default"`
		} `yaml:"inputs"`
	}

	if err := yaml.Unmarshal(actionData, &actionMeta); err != nil {
		return fmt.Errorf("failed to parse action metadata: %w", err)
	}

	fmt.Printf("      Action type: %s\n", actionMeta.Runs.Using)

	// Handle different action types
	switch actionMeta.Runs.Using {
	case "composite":
		return executeCompositeAction(&actionMeta, step, jobDir, runnerImage, config, actionDir, stepsDir)
	case "node20", "node16", "node12":
		return executeNodeAction(&actionMeta, step, jobDir, runnerImage, config, actionDir)
	case "docker":
		return fmt.Errorf("docker actions not supported yet")
	default:
		return fmt.Errorf("unsupported action type: %s", actionMeta.Runs.Using)
	}
}

// executeCompositeAction executes a composite action
func executeCompositeAction(actionMeta interface{}, step *Step, jobDir, runnerImage string, config *Config, actionDir, stepsDir string) error {
	meta := actionMeta.(*struct {
		Runs struct {
			Using string `yaml:"using"`
			Main  string `yaml:"main"`
			Steps []struct {
				Name string                 `yaml:"name"`
				Run  string                 `yaml:"run"`
				Uses string                 `yaml:"uses"`
				With map[string]interface{} `yaml:"with"`
				Env  map[string]string      `yaml:"env"`
				ID   string                 `yaml:"id"`
			} `yaml:"steps"`
		} `yaml:"runs"`
		Inputs map[string]struct {
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
			Default     string `yaml:"default"`
		} `yaml:"inputs"`
	})

	// Prepare environment with input variables
	actionEnv := make(map[string]string)

	// Set inputs from step.With
	inputs := make(map[string]interface{})
	if step.With != nil {
		for inputName, value := range step.With {
			// Expand environment variables in the value
			expandedValue := expandEnvironmentVariables(fmt.Sprintf("%v", value))
			envName := fmt.Sprintf("INPUT_%s", strings.ToUpper(strings.ReplaceAll(inputName, "-", "_")))
			actionEnv[envName] = expandedValue
			inputs[inputName] = expandedValue
		}
	}

	// Track step outputs
	stepOutputs := make(map[string]map[string]string)

	// Execute each step in the composite action
	for i, actionStep := range meta.Runs.Steps {
		fmt.Printf("        Action Step %d: %s\n", i+1, actionStep.Name)

		// Substitute templates in run command and name
		substitutedRun := substituteActionTemplates(actionStep.Run, inputs, stepOutputs)
		substitutedName := substituteActionTemplates(actionStep.Name, inputs, stepOutputs)

		// Create step with combined environment
		combinedEnv := make(map[string]string)
		for k, v := range actionEnv {
			combinedEnv[k] = v
		}
		for k, v := range actionStep.Env {
			combinedEnv[k] = substituteActionTemplates(v, inputs, stepOutputs)
		}

		stepToExecute := &Step{
			Name: substitutedName,
			Run:  substitutedRun,
			Uses: actionStep.Uses,
			With: actionStep.With,
			Env:  combinedEnv,
		}

		if actionStep.Run != "" {
			// Mount both job directory and action directory
			if err := executeActionRunStep(stepToExecute, jobDir, runnerImage, config, actionDir); err != nil {
				return fmt.Errorf("action step %d failed: %w", i+1, err)
			}

			// If step has an ID, capture its outputs
			if actionStep.ID != "" {
				githubOutputPath := filepath.Join(jobDir, "github_output.txt")
				outputs, err := parseStepOutputs(githubOutputPath)
				if err != nil {
					fmt.Printf("        Warning: failed to parse step outputs: %v\n", err)
				} else {
					stepOutputs[actionStep.ID] = outputs
					fmt.Printf("        Step outputs: %v\n", outputs)
				}

				// Clear the output file for next step
				if err := os.WriteFile(githubOutputPath, []byte(""), 0644); err != nil {
					fmt.Printf("        Warning: failed to clear output file: %v\n", err)
				}
			}
		} else if actionStep.Uses != "" {
			// Recursive action call
			if err := executeAction(stepToExecute, jobDir, runnerImage, config, stepsDir); err != nil {
				return fmt.Errorf("nested action step %d failed: %w", i+1, err)
			}
		}
	}

	return nil
}

// executeNodeAction executes a Node.js action
func executeNodeAction(actionMeta interface{}, step *Step, jobDir, runnerImage string, config *Config, actionDir string) error {
	meta := actionMeta.(*struct {
		Runs struct {
			Using string `yaml:"using"`
			Main  string `yaml:"main"`
			Steps []struct {
				Name string                 `yaml:"name"`
				Run  string                 `yaml:"run"`
				Uses string                 `yaml:"uses"`
				With map[string]interface{} `yaml:"with"`
				Env  map[string]string      `yaml:"env"`
				ID   string                 `yaml:"id"`
			} `yaml:"steps"`
		} `yaml:"runs"`
		Inputs map[string]struct {
			Description string `yaml:"description"`
			Required    bool   `yaml:"required"`
			Default     string `yaml:"default"`
		} `yaml:"inputs"`
	})

	// Prepare environment with input variables
	env := make([]string, 0)

	// Add config environment variables (these should include all required GitHub Actions variables)
	for key, value := range config.Env {
		env = append(env, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Set inputs from step.With
	if step.With != nil {
		for inputName, value := range step.With {
			// Expand environment variables in the value
			expandedValue := expandEnvironmentVariables(fmt.Sprintf("%v", value))
			envName := fmt.Sprintf("INPUT_%s", strings.ToUpper(strings.ReplaceAll(inputName, "-", "_")))
			env = append(env, "-e", fmt.Sprintf("%s=%s", envName, expandedValue))
		}
	}

	// Build docker run command with both workspace and action mounted
	args := []string{
		"run", "--rm",
		"--network", "host", // Enable network access for GitHub operations
		"-v", fmt.Sprintf("%s:/workspace", jobDir),
		"-v", fmt.Sprintf("%s:/action", actionDir),
		"--workdir", "/workspace",
	}

	// Add environment variables
	args = append(args, env...)

	// Add image
	args = append(args, runnerImage)

	// Run node with the action's main file
	mainFile := meta.Runs.Main
	if mainFile == "" {
		mainFile = "index.js"
	}
	args = append(args, "node", filepath.Join("/action", mainFile))

	// Execute command
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// executeActionRunStep executes a run step within an action context
func executeActionRunStep(step *Step, jobDir, runnerImage string, config *Config, actionDir string) error {
	// Create GitHub Actions environment files
	githubOutputPath := filepath.Join(jobDir, "github_output.txt")
	githubEnvPath := filepath.Join(jobDir, "github_env.txt")

	// Create empty files if they don't exist
	if _, err := os.Stat(githubOutputPath); os.IsNotExist(err) {
		if err := os.WriteFile(githubOutputPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create GITHUB_OUTPUT file: %w", err)
		}
	}
	if _, err := os.Stat(githubEnvPath); os.IsNotExist(err) {
		if err := os.WriteFile(githubEnvPath, []byte(""), 0644); err != nil {
			return fmt.Errorf("failed to create GITHUB_ENV file: %w", err)
		}
	}

	// Prepare environment variables
	env := make([]string, 0)

	// Add config environment variables
	for key, value := range config.Env {
		env = append(env, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add GitHub Actions environment files
	env = append(env, "-e", "GITHUB_OUTPUT=/workspace/github_output.txt")
	env = append(env, "-e", "GITHUB_ENV=/workspace/github_env.txt")

	// Add step-specific environment variables
	for key, value := range step.Env {
		env = append(env, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Build docker run command with both workspace and action mounted
	args := []string{
		"run", "--rm",
		"--network", "host", // Enable network access for GitHub operations
		"-v", fmt.Sprintf("%s:/workspace", jobDir),
		"-v", fmt.Sprintf("%s:/action", actionDir),
		"--workdir", "/action",
	}

	// Add environment variables
	args = append(args, env...)

	// Add image
	args = append(args, runnerImage)

	// Add shell command
	args = append(args, "bash", "-c", step.Run)

	// Execute command
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func executeWorkflow(workflow *Workflow, config *Config) error {
	fmt.Printf("Executing workflow: %s\n", workflow.Name)

	// Create pipeline temp directory
	pipelineDir, err := createPipelineDir(workflow.Name)
	if err != nil {
		return fmt.Errorf("failed to create pipeline directory: %w", err)
	}
	defer func() {
		if removeErr := os.RemoveAll(pipelineDir); removeErr != nil {
			fmt.Printf("Warning: failed to cleanup pipeline directory %s: %v\n", pipelineDir, removeErr)
		}
	}()

	// Expand matrix jobs
	expandedJobs := expandMatrixJobs(workflow.Jobs)

	// Build dependency graph and execute jobs
	return executeJobs(expandedJobs, config, pipelineDir)
}

func createPipelineDir(workflowName string) (string, error) {
	// Generate random suffix
	suffix := fmt.Sprintf("%06d", rand.Intn(1000000))

	// Clean workflow name for directory
	cleanName := strings.ReplaceAll(strings.ToLower(workflowName), " ", "-")
	dirName := fmt.Sprintf("%s-%s", cleanName, suffix)

	pipelineDir := filepath.Join("/tmp", dirName)
	return pipelineDir, os.MkdirAll(pipelineDir, 0755)
}

func executeJobs(jobs map[string]*Job, config *Config, pipelineDir string) error {
	// Create steps directory for actions
	stepsDir := filepath.Join(pipelineDir, "steps")
	if err := os.MkdirAll(stepsDir, 0755); err != nil {
		return fmt.Errorf("failed to create steps directory: %w", err)
	}

	// Simple execution for now - we'll implement dependency resolution later
	for jobName, job := range jobs {
		fmt.Printf("Job: %s\n", jobName)
		fmt.Printf("  Runs on: %v\n", job.RunsOn)
		fmt.Printf("  Steps: %d\n", len(job.Steps))

		// Create job directory
		jobDir := filepath.Join(pipelineDir, jobName)
		if err := os.MkdirAll(jobDir, 0755); err != nil {
			return fmt.Errorf("failed to create job directory: %w", err)
		}

		// Get runner image
		runnerImage, err := getRunnerImage(job.RunsOn)
		if err != nil {
			return fmt.Errorf("failed to get runner image: %w", err)
		}

		// Execute steps in container
		if err := executeJobSteps(job, jobDir, runnerImage, config, stepsDir); err != nil {
			return fmt.Errorf("job %s failed: %w", jobName, err)
		}
	}

	return nil
}

func getRunnerImage(runsOn interface{}) (string, error) {
	var runners []string

	switch v := runsOn.(type) {
	case string:
		runners = []string{v}
	case []string:
		runners = v
	case []interface{}:
		for _, item := range v {
			if str, ok := item.(string); ok {
				runners = append(runners, str)
			}
		}
	default:
		return "", fmt.Errorf("invalid runs-on type: %T", runsOn)
	}

	if len(runners) == 0 {
		return "", fmt.Errorf("no runs-on specified")
	}

	// Map GitHub runner names to our runner images
	runnerMap := map[string]string{
		"ubuntu-latest": "ubuntu-latest",
		"ubuntu-22.04":  "ubuntu-22.04",
		"ubuntu-20.04":  "ubuntu-20.04",
		"debian-latest": "debian-latest",
		"debian-12":     "debian-12",
		"debian-11":     "debian-11",
		"alpine-latest": "alpine-latest",
	}

	runner := runners[0] // Use first runner
	if dockerfileName, ok := runnerMap[runner]; ok {
		imageName := fmt.Sprintf("vermont-runner:%s", dockerfileName)

		// Build the image if it doesn't exist
		if err := buildRunnerImage(dockerfileName, imageName); err != nil {
			return "", fmt.Errorf("failed to build runner image: %w", err)
		}

		return imageName, nil
	}

	// Fall back to ubuntu-latest for unsupported runners
	fmt.Printf("  Warning: unsupported runner '%s', falling back to ubuntu-latest\n", runner)
	imageName := fmt.Sprintf("vermont-runner:%s", "ubuntu-latest")

	// Build the image if it doesn't exist
	if err := buildRunnerImage("ubuntu-latest", imageName); err != nil {
		return "", fmt.Errorf("failed to build fallback runner image: %w", err)
	}

	return imageName, nil
}

func buildRunnerImage(dockerfileName, imageName string) error {
	// Check if image exists
	checkCmd := exec.Command("docker", "images", "-q", imageName)
	output, _ := checkCmd.Output()
	if len(strings.TrimSpace(string(output))) > 0 {
		fmt.Printf("  Container: %s (exists)\n", imageName)
		return nil // Image already exists
	}

	fmt.Printf("  Building container: %s\n", imageName)

	// Build the image
	dockerfilePath := filepath.Join("runners", fmt.Sprintf("Dockerfile.%s", dockerfileName))
	buildCmd := exec.Command("docker", "build", "-f", dockerfilePath, "-t", imageName, ".")
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr

	if err := buildCmd.Run(); err != nil {
		return fmt.Errorf("docker build failed: %w", err)
	}

	return nil
}

func executeJobSteps(job *Job, jobDir, runnerImage string, config *Config, stepsDir string) error {
	for i, step := range job.Steps {
		stepNum := i + 1
		if step.Name != "" {
			fmt.Printf("    Step %d: %s\n", stepNum, step.Name)
		} else {
			fmt.Printf("    Step %d\n", stepNum)
		}

		if step.Run != "" {
			// Execute shell command in container
			if err := executeRunStep(step, jobDir, runnerImage, config); err != nil {
				return fmt.Errorf("step %d failed: %w", stepNum, err)
			}
		} else if step.Uses != "" {
			// Execute GitHub Action
			if err := executeAction(step, jobDir, runnerImage, config, stepsDir); err != nil {
				return fmt.Errorf("step %d failed: %w", stepNum, err)
			}
		}
	}

	return nil
}

func executeRunStep(step *Step, jobDir, runnerImage string, config *Config) error {
	// Prepare environment variables
	env := make([]string, 0)

	// Add config environment variables
	for key, value := range config.Env {
		env = append(env, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add step-specific environment variables
	for key, value := range step.Env {
		env = append(env, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Build docker run command
	args := []string{
		"run", "--rm",
		"--network", "host", // Enable network access for GitHub operations
		"-v", fmt.Sprintf("%s:/workspace", jobDir),
		"--workdir", "/workspace",
	}

	// Add environment variables
	args = append(args, env...)

	// Add image
	args = append(args, runnerImage)

	// Add shell command
	args = append(args, "bash", "-c", step.Run)

	// Execute command
	cmd := exec.Command("docker", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
