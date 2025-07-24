package executor

import (
	"context"
	"fmt"

	"github.com/polatengin/vermont/pkg/workflow"
)

// Executor handles workflow execution
type Executor struct {
	config interface{} // Will be replaced with proper config type
	logger interface{} // Will be replaced with proper logger interface
}

// New creates a new executor
func New(config interface{}, logger interface{}) *Executor {
	return &Executor{
		config: config,
		logger: logger,
	}
}

// Execute executes a workflow
func (e *Executor) Execute(ctx context.Context, wf *workflow.Workflow) error {
	// TODO: Implement workflow execution logic
	fmt.Printf("Executing workflow: %s\n", wf.Name)

	// For now, just print job information
	for jobID, job := range wf.Jobs {
		fmt.Printf("Job: %s\n", jobID)
		fmt.Printf("  Runs on: %v\n", job.GetRunsOn())
		fmt.Printf("  Steps: %d\n", len(job.Steps))

		for i, step := range job.Steps {
			if step.Name != "" {
				fmt.Printf("    Step %d: %s\n", i+1, step.Name)
			} else {
				fmt.Printf("    Step %d\n", i+1)
			}

			if step.Run != "" {
				fmt.Printf("      Run: %s\n", step.Run)
			}

			if step.Uses != "" {
				fmt.Printf("      Uses: %s\n", step.Uses)
			}
		}
	}

	return nil
}
