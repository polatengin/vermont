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
	config *config.Config
	logger *logger.Logger
}

// New creates a new executor
func New(cfg *config.Config, log *logger.Logger) *Executor {
	return &Executor{
		config: cfg,
		logger: log,
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
			// TODO: Implement action execution
			e.logger.Warn("Action execution not yet implemented", "action", step.Uses)
			continue
		}

		if step.Run != "" {
			result := e.executeStep(ctx, step, jobEnv)
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
				"duration", result.Duration)
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
