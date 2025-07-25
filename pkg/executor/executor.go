package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	Outputs     map[string]string
}

// JobState represents the state of a job
type JobState struct {
	ID           string
	Job          *workflow.Job
	Status       JobStatus
	Result       *JobResult
	Dependencies []string
	StartTime    time.Time
	EndTime      time.Time
}

// JobStatus represents the execution status of a job
type JobStatus int

const (
	JobStatusPending JobStatus = iota
	JobStatusReady
	JobStatusRunning
	JobStatusCompleted
	JobStatusFailed
	JobStatusSkipped
)

// JobScheduler manages job execution with dependency handling
type JobScheduler struct {
	executor      *Executor
	jobs          map[string]*JobState
	completedJobs map[string]bool
	mutex         sync.RWMutex
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

	executor := &Executor{
		config:           cfg,
		logger:           log,
		containerManager: container.NewManager(cfg, log),
		actionManager:    actionManager,
		actionExecutor:   actionExecutor,
	}

	// Ensure work directory exists and has proper permissions
	executor.ensureWorkDirectory()

	return executor
}

// Execute executes a workflow with dependency management and parallel execution
func (e *Executor) Execute(ctx context.Context, wf *workflow.Workflow) error {
	e.logger.Info("Executing workflow", "name", wf.Name)
	fmt.Printf("Executing workflow: %s\n", wf.Name)

	// Create and run job scheduler
	scheduler := NewJobScheduler(e)
	return scheduler.ExecuteWorkflow(ctx, wf)
}

// NewJobScheduler creates a new job scheduler
func NewJobScheduler(executor *Executor) *JobScheduler {
	return &JobScheduler{
		executor:      executor,
		jobs:          make(map[string]*JobState),
		completedJobs: make(map[string]bool),
	}
}

// ExecuteWorkflow executes all jobs in a workflow with dependency management
func (s *JobScheduler) ExecuteWorkflow(ctx context.Context, wf *workflow.Workflow) error {
	// Initialize job states
	s.initializeJobs(wf)

	// Validate dependencies
	if err := s.validateDependencies(); err != nil {
		return fmt.Errorf("dependency validation failed: %w", err)
	}

	// Execute jobs
	return s.executeJobs(ctx)
}

// initializeJobs creates JobState for each job
func (s *JobScheduler) initializeJobs(wf *workflow.Workflow) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	for jobID, job := range wf.Jobs {
		s.jobs[jobID] = &JobState{
			ID:           jobID,
			Job:          job,
			Status:       JobStatusPending,
			Dependencies: job.GetDependencies(),
		}
	}
}

// validateDependencies checks for circular dependencies and missing jobs
func (s *JobScheduler) validateDependencies() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	// Check for missing dependencies
	for jobID, jobState := range s.jobs {
		for _, depID := range jobState.Dependencies {
			if _, exists := s.jobs[depID]; !exists {
				return fmt.Errorf("job '%s' depends on non-existent job '%s'", jobID, depID)
			}
		}
	}

	// Check for circular dependencies using DFS
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for jobID := range s.jobs {
		if !visited[jobID] {
			if s.hasCycle(jobID, visited, recStack) {
				return fmt.Errorf("circular dependency detected involving job '%s'", jobID)
			}
		}
	}

	return nil
}

// hasCycle performs DFS to detect circular dependencies
func (s *JobScheduler) hasCycle(jobID string, visited, recStack map[string]bool) bool {
	visited[jobID] = true
	recStack[jobID] = true

	jobState := s.jobs[jobID]
	for _, depID := range jobState.Dependencies {
		if !visited[depID] {
			if s.hasCycle(depID, visited, recStack) {
				return true
			}
		} else if recStack[depID] {
			return true
		}
	}

	recStack[jobID] = false
	return false
}

// executeJobs executes jobs with dependency management and parallel execution
func (s *JobScheduler) executeJobs(ctx context.Context) error {
	var wg sync.WaitGroup
	errorChan := make(chan error, len(s.jobs))
	maxConcurrent := s.executor.config.Runner.MaxConcurrentJobs
	semaphore := make(chan struct{}, maxConcurrent)

	// Continue until all jobs are completed or failed
	for len(s.completedJobs) < len(s.jobs) {
		readyJobs := s.getReadyJobs()
		if len(readyJobs) == 0 {
			// Check if we're in a deadlock (no ready jobs but not all completed)
			if len(s.completedJobs) < len(s.jobs) {
				pendingJobs := s.getPendingJobs()
				if len(pendingJobs) > 0 {
					return fmt.Errorf("workflow deadlock detected: no ready jobs but %d jobs still pending", len(pendingJobs))
				}
			}
			break
		}

		// Start ready jobs
		for _, jobState := range readyJobs {
			wg.Add(1)
			go func(js *JobState) {
				defer wg.Done()

				// Acquire semaphore
				semaphore <- struct{}{}
				defer func() { <-semaphore }()

				// Execute job
				err := s.executeJob(ctx, js)
				if err != nil {
					errorChan <- err
				}
			}(jobState)
		}

		// Wait for current batch to complete
		wg.Wait()

		// Check for errors
		select {
		case err := <-errorChan:
			return err
		default:
		}

		// Small delay to prevent busy waiting
		time.Sleep(10 * time.Millisecond)
	}

	s.executor.logger.Info("Workflow execution completed", "totalJobs", len(s.jobs), "completedJobs", len(s.completedJobs))
	return nil
}

// getReadyJobs returns jobs that are ready to execute
func (s *JobScheduler) getReadyJobs() []*JobState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var ready []*JobState
	for _, jobState := range s.jobs {
		if jobState.Status == JobStatusPending && s.areDependenciesMet(jobState) {
			jobState.Status = JobStatusReady
			ready = append(ready, jobState)
		}
	}
	return ready
}

// getPendingJobs returns jobs that are still pending
func (s *JobScheduler) getPendingJobs() []*JobState {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var pending []*JobState
	for _, jobState := range s.jobs {
		if jobState.Status == JobStatusPending {
			pending = append(pending, jobState)
		}
	}
	return pending
}

// areDependenciesMet checks if all dependencies for a job are completed successfully
func (s *JobScheduler) areDependenciesMet(jobState *JobState) bool {
	for _, depID := range jobState.Dependencies {
		if !s.completedJobs[depID] {
			return false
		}

		// Check if dependency succeeded
		if depState, exists := s.jobs[depID]; exists {
			if depState.Status == JobStatusFailed {
				return false
			}
		}
	}
	return true
}

// executeJob executes a single job and updates its state
func (s *JobScheduler) executeJob(ctx context.Context, jobState *JobState) error {
	s.mutex.Lock()
	jobState.Status = JobStatusRunning
	jobState.StartTime = time.Now()
	s.mutex.Unlock()

	s.executor.logger.Info("Starting job", "job", jobState.ID, "dependencies", jobState.Dependencies)

	// Execute the job using the existing executeJob method
	err := s.executor.executeJob(ctx, jobState.ID, jobState.Job)

	s.mutex.Lock()
	defer s.mutex.Unlock()

	jobState.EndTime = time.Now()
	if err != nil {
		jobState.Status = JobStatusFailed
		s.executor.logger.Error("Job failed", "job", jobState.ID, "error", err, "duration", jobState.EndTime.Sub(jobState.StartTime))
		return fmt.Errorf("job %s failed: %w", jobState.ID, err)
	} else {
		jobState.Status = JobStatusCompleted
		s.completedJobs[jobState.ID] = true
		s.executor.logger.Info("Job completed", "job", jobState.ID, "duration", jobState.EndTime.Sub(jobState.StartTime))
	}

	return nil
}

// executeJob executes a single job
func (e *Executor) executeJob(ctx context.Context, jobID string, job *workflow.Job) error {
	e.logger.Info("Starting job", "job", jobID)
	fmt.Printf("Job: %s\n", jobID)

	// Show dependencies if any
	dependencies := job.GetDependencies()
	if len(dependencies) > 0 {
		fmt.Printf("  Needs: %v\n", dependencies)
	}

	fmt.Printf("  Runs on: %v\n", job.GetRunsOn())
	fmt.Printf("  Steps: %d\n", len(job.Steps))

	// Set up job environment
	jobEnv := e.createJobEnvironment(jobID, job)

	// Check if Docker is available (required for all execution)
	if !e.containerManager.IsDockerAvailable(ctx) {
		return fmt.Errorf("Docker is required but not available. Please install and start Docker")
	}

	// Always use container execution
	containerImage := e.containerManager.GetDefaultImage(job.GetRunsOn())
	fmt.Printf("  Container: %s\n", containerImage)

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

			// Pass container context to action executor (always using containers)
			var result *actions.ActionExecutionResult
			var err error

			// For container execution, we need to inform the action executor
			// about the container environment
			containerEnv := make(map[string]string)
			for k, v := range jobEnv {
				containerEnv[k] = v
			}
			containerEnv["_VERMONT_CONTAINER_MODE"] = "true"
			containerEnv["_VERMONT_CONTAINER_IMAGE"] = containerImage
			containerEnv["_VERMONT_WORK_DIR"] = e.config.Runner.WorkDir

			result, err = e.actionExecutor.Execute(ctx, step.Uses, inputs, containerEnv)

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

			// Execute in container (always)
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
				"container", true)
		}
	}

	e.logger.Info("Job completed", "job", jobID)
	return nil
}

// createJobEnvironment creates environment variables for a job
func (e *Executor) createJobEnvironment(jobID string, job *workflow.Job) map[string]string {
	env := make(map[string]string)

	// Start with environment variables from config
	for key, value := range e.config.GetEnvironmentVariables() {
		env[key] = value
	}

	// Get Git information from the current repository
	gitInfo := e.getGitInfo()

	// Default GitHub Actions environment variables
	env["GITHUB_WORKFLOW"] = jobID
	env["GITHUB_JOB"] = jobID
	env["GITHUB_ACTION"] = ""
	env["GITHUB_ACTOR"] = "vermont-runner"

	// Set Git-based environment variables
	env["GITHUB_REPOSITORY"] = gitInfo["GITHUB_REPOSITORY"]
	env["GITHUB_EVENT_NAME"] = "push"
	env["GITHUB_SHA"] = gitInfo["GITHUB_SHA"]
	env["GITHUB_REF"] = gitInfo["GITHUB_REF"]
	env["GITHUB_HEAD_REF"] = gitInfo["GITHUB_HEAD_REF"]
	env["GITHUB_BASE_REF"] = gitInfo["GITHUB_BASE_REF"]
	env["RUNNER_OS"] = "Linux"
	env["RUNNER_ARCH"] = "X64"
	env["RUNNER_NAME"] = "Vermont Runner"
	env["RUNNER_TOOL_CACHE"] = "/opt/hostedtoolcache"

	// Add job-specific environment variables (these can override config env vars)
	for key, value := range job.Env {
		env[key] = value
	}

	return env
}

// getGitInfo reads Git information from the current repository
func (e *Executor) getGitInfo() map[string]string {
	gitInfo := make(map[string]string)

	// Get current commit SHA
	if cmd := exec.Command("git", "rev-parse", "HEAD"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			gitInfo["GITHUB_SHA"] = strings.TrimSpace(string(output))
		}
	}

	// Get current branch name
	if cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			branchName := strings.TrimSpace(string(output))
			if branchName != "" && branchName != "HEAD" {
				gitInfo["GITHUB_REF"] = "refs/heads/" + branchName
			}
		}
	}

	// Get remote origin URL to determine repository
	if cmd := exec.Command("git", "config", "--get", "remote.origin.url"); cmd != nil {
		if output, err := cmd.Output(); err == nil {
			remoteURL := strings.TrimSpace(string(output))
			if repo := parseGitHubRepository(remoteURL); repo != "" {
				gitInfo["GITHUB_REPOSITORY"] = repo
			}
		}
	}

	// For pull requests, try to get head and base refs
	// This is more complex and would require additional logic to detect PR context
	// For now, we'll leave them empty as they're mainly used in PR workflows

	return gitInfo
}

// parseGitHubRepository extracts owner/repo from a GitHub remote URL
func parseGitHubRepository(remoteURL string) string {
	// Handle both HTTPS and SSH formats
	// HTTPS: https://github.com/owner/repo.git
	// SSH: git@github.com:owner/repo.git

	if strings.Contains(remoteURL, "github.com") {
		// Remove .git suffix if present
		remoteURL = strings.TrimSuffix(remoteURL, ".git")

		if strings.HasPrefix(remoteURL, "https://github.com/") {
			return strings.TrimPrefix(remoteURL, "https://github.com/")
		} else if strings.HasPrefix(remoteURL, "git@github.com:") {
			return strings.TrimPrefix(remoteURL, "git@github.com:")
		}
	}

	return ""
}

// ensureWorkDirectory ensures the work directory exists and has proper permissions
func (e *Executor) ensureWorkDirectory() {
	workDir := e.config.Runner.WorkDir
	if workDir == "" {
		return
	}

	// Create directory if it doesn't exist
	if err := os.MkdirAll(workDir, 0755); err != nil {
		e.logger.Warn("Failed to create work directory", "dir", workDir, "error", err)
		return
	}

	// Try to make it writable by the current user
	// This is a best-effort attempt - container execution might still fail
	// if the container user has a different UID
	if err := os.Chmod(workDir, 0755); err != nil {
		e.logger.Warn("Failed to set work directory permissions", "dir", workDir, "error", err)
	}

	e.logger.Debug("Work directory ensured", "dir", workDir)
}

