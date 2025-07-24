package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/polatengin/vermont/internal/config"
	"github.com/polatengin/vermont/internal/logger"
	"github.com/polatengin/vermont/pkg/actions"
	"github.com/polatengin/vermont/pkg/container"
	"github.com/polatengin/vermont/pkg/workflow"
)

// StepResult represents the result of a step execution
type StepResult struct {
	Success  bool
	Output   string
	Error    string
	Duration time.Duration
}

// JobResult represents the result of a job execution
type JobResult struct {
	Success     bool
	Steps       []StepResult
	Duration    time.Duration
	Environment map[string]string
}

// Executor handles workflow execution
type Executor struct {
	config           *config.Config
	logger           *logger.Logger
	containerManager *container.Manager
	actionManager    *actions.Manager
	actionExecutor   *actions.Executor
}

// New creates a new executor
func New(cfg *config.Config, log *logger.Logger) *Executor {
	actionManager := actions.NewManager(cfg, log)
	actionExecutor := actions.NewExecutor(actionManager, log)

	return &Executor{
		config:           cfg,
		logger:           log,
		containerManager: container.NewManager(cfg, log),
		actionManager:    actionManager,
		actionExecutor:   actionExecutor,
	}
}

// Execute executes a workflow
func (e *Executor) Execute(ctx context.Context, wf *workflow.Workflow) error {
	e.logger.Info("Executing workflow", "name", wf.Name)
	fmt.Printf("Executing workflow: %s\n", wf.Name)

	// Execute jobs (for now, sequentially - TODO: handle dependencies)
	for jobID, job := range wf.Jobs {
		if err := e.executeJob(ctx, jobID, job); err != nil {
			e.logger.Error("Job execution failed", "job", jobID, "error", err)
			return fmt.Errorf("job %s failed: %w", jobID, err)
		}
	}

	e.logger.Info("Workflow execution completed", "name", wf.Name)
	return nil
}

// executeJob executes a single job
func (e *Executor) executeJob(ctx context.Context, jobID string, job *workflow.Job) error {
	e.logger.Info("Starting job", "job", jobID)
	fmt.Printf("Job: %s\n", jobID)
	fmt.Printf("  Runs on: %v\n", job.GetRunsOn())
	fmt.Printf("  Steps: %d\n", len(job.Steps))

	// Set up job environment
	jobEnv := e.createJobEnvironment(jobID, job)

	// Determine if we should use containers
	useContainer := e.shouldUseContainer(job)
	if useContainer {
		// Check if Docker is available
		if !e.containerManager.IsDockerAvailable(ctx) {
			e.logger.Warn("Docker not available, falling back to host execution")
			useContainer = false
		}
	}

	var containerImage string
	if useContainer {
		containerImage = e.containerManager.GetDefaultImage(job.GetRunsOn())
		fmt.Printf("  Container: %s\n", containerImage)
	} else {
		fmt.Printf("  Execution: Host\n")
	}

	// Execute steps
	for i, step := range job.Steps {
		stepNum := i + 1
		if step.Name != "" {
			fmt.Printf("    Step %d: %s\n", stepNum, step.Name)
		} else {
			fmt.Printf("    Step %d\n", stepNum)
		}

		if step.Uses != "" {
			fmt.Printf("      Uses: %s\n", step.Uses)

			// Convert step.With to map[string]interface{}
			inputs := make(map[string]interface{})
			for k, v := range step.With {
				inputs[k] = v
			}

			// Execute the action
			start := time.Now()
			result, err := e.actionExecutor.Execute(ctx, step.Uses, inputs, jobEnv)
			duration := time.Since(start)

			if err != nil {
				e.logger.Error("Action execution failed",
					"job", jobID,
					"step", stepNum,
					"action", step.Uses,
					"error", err)
				return fmt.Errorf("step %d (action %s) failed: %v", stepNum, step.Uses, err)
			}

			if !result.Success {
				e.logger.Error("Action returned failure",
					"job", jobID,
					"step", stepNum,
					"action", step.Uses,
					"error", result.Error)
				return fmt.Errorf("step %d (action %s) failed: %s", stepNum, step.Uses, result.Error)
			}

			// Show action outputs if any
			if len(result.Outputs) > 0 {
				fmt.Printf("      Outputs:\n")
				for key, value := range result.Outputs {
					fmt.Printf("        %s: %s\n", key, value)
				}
			}

			e.logger.Info("Action completed",
				"job", jobID,
				"step", stepNum,
				"action", step.Uses,
				"duration", duration)
			continue
		}

		if step.Run != "" {
			var result StepResult

			if useContainer {
				// Execute in container
				containerResult, containerErr := e.containerManager.RunStep(ctx, step, containerImage, jobEnv, e.config.Runner.WorkDir)
				if containerErr != nil {
					e.logger.Error("Container step execution failed",
						"job", jobID,
						"step", stepNum,
						"error", containerErr)
					return fmt.Errorf("step %d failed: %v", stepNum, containerErr)
				}

				// Convert container result to step result
				result = StepResult{
					Success:  containerResult.Success,
					Output:   containerResult.Output,
					Error:    containerResult.Error,
					Duration: containerResult.Duration,
				}
			} else {
				// Execute on host
				result = e.executeStep(ctx, step, jobEnv)
			}

			if !result.Success {
				e.logger.Error("Step execution failed",
					"job", jobID,
					"step", stepNum,
					"error", result.Error)
				return fmt.Errorf("step %d failed: %s", stepNum, result.Error)
			}

			// Show step output if there's any
			if strings.TrimSpace(result.Output) != "" {
				fmt.Printf("      Output: %s", result.Output)
			}

			e.logger.Info("Step completed",
				"job", jobID,
				"step", stepNum,
				"duration", result.Duration,
				"container", useContainer)
		}
	}

	e.logger.Info("Job completed", "job", jobID)
	return nil
}

// executeStep executes a single step
func (e *Executor) executeStep(ctx context.Context, step *workflow.Step, env map[string]string) StepResult {
	start := time.Now()

	// Parse the command
	commands := strings.Split(strings.TrimSpace(step.Run), "\n")

	var output strings.Builder

	for _, command := range commands {
		command = strings.TrimSpace(command)
		if command == "" {
			continue
		}

		// Execute the command
		cmd := exec.CommandContext(ctx, "bash", "-c", command)

		// Set environment variables
		cmd.Env = os.Environ()
		for key, value := range env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}

		// Set environment variables from step
		for key, value := range step.Env {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
		}

		// Execute command
		out, err := cmd.CombinedOutput()
		output.WriteString(string(out))

		if err != nil {
			e.logger.Error("Command execution failed",
				"command", command,
				"error", err,
				"output", string(out))

			return StepResult{
				Success:  false,
				Output:   output.String(),
				Error:    fmt.Sprintf("Command failed: %s - %v", command, err),
				Duration: time.Since(start),
			}
		}

		e.logger.Debug("Command executed",
			"command", command,
			"output", string(out))
	}

	return StepResult{
		Success:  true,
		Output:   output.String(),
		Error:    "",
		Duration: time.Since(start),
	}
}

// createJobEnvironment creates environment variables for a job
func (e *Executor) createJobEnvironment(jobID string, job *workflow.Job) map[string]string {
	env := make(map[string]string)

	// Default GitHub Actions environment variables
	env["GITHUB_WORKFLOW"] = jobID
	env["GITHUB_JOB"] = jobID
	env["GITHUB_ACTION"] = ""
	env["GITHUB_ACTOR"] = "vermont-runner"
	env["GITHUB_REPOSITORY"] = "local/repository"
	env["GITHUB_EVENT_NAME"] = "push"
	env["GITHUB_SHA"] = "abc123"
	env["GITHUB_REF"] = "refs/heads/main"
	env["GITHUB_HEAD_REF"] = ""
	env["GITHUB_BASE_REF"] = ""
	env["RUNNER_OS"] = "Linux"
	env["RUNNER_ARCH"] = "X64"
	env["RUNNER_NAME"] = "Vermont Runner"
	env["RUNNER_TOOL_CACHE"] = "/opt/hostedtoolcache"

	// Add job-specific environment variables
	for key, value := range job.Env {
		env[key] = value
	}

	return env
}

// shouldUseContainer determines if a job should run in a container
func (e *Executor) shouldUseContainer(job *workflow.Job) bool {
	// Check configuration setting
	if e.config.Container.Runtime == "" || e.config.Container.Runtime == "none" {
		return false
	}

	// For now, use container execution if Docker is configured
	// In the future, this could be more sophisticated based on job requirements
	return e.config.Container.Runtime == "docker"
}
