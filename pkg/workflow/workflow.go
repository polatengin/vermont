package workflow

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Workflow represents a GitHub Actions workflow
type Workflow struct {
	Name     string            `yaml:"name"`
	On       interface{}       `yaml:"on"` // Can be string, array, or object
	Jobs     map[string]*Job   `yaml:"jobs"`
	Env      map[string]string `yaml:"env,omitempty"`
	Defaults *Defaults         `yaml:"defaults,omitempty"`
}

// Job represents a workflow job
type Job struct {
	Name           string                 `yaml:"name,omitempty"`
	RunsOn         interface{}            `yaml:"runs-on"`         // Can be string or array
	Needs          interface{}            `yaml:"needs,omitempty"` // Can be string or array
	If             string                 `yaml:"if,omitempty"`
	Steps          []*Step                `yaml:"steps"`
	Env            map[string]string      `yaml:"env,omitempty"`
	Strategy       *Strategy              `yaml:"strategy,omitempty"`
	Container      interface{}            `yaml:"container,omitempty"` // Can be string or object
	Services       map[string]interface{} `yaml:"services,omitempty"`
	Outputs        map[string]string      `yaml:"outputs,omitempty"`
	TimeoutMinutes int                    `yaml:"timeout-minutes,omitempty"`
	Defaults       *Defaults              `yaml:"defaults,omitempty"`
}

// Step represents a workflow step
type Step struct {
	ID               string                 `yaml:"id,omitempty"`
	Name             string                 `yaml:"name,omitempty"`
	Run              string                 `yaml:"run,omitempty"`
	Uses             string                 `yaml:"uses,omitempty"`
	With             map[string]interface{} `yaml:"with,omitempty"`
	Env              map[string]string      `yaml:"env,omitempty"`
	If               string                 `yaml:"if,omitempty"`
	Shell            string                 `yaml:"shell,omitempty"`
	WorkingDirectory string                 `yaml:"working-directory,omitempty"`
	TimeoutMinutes   int                    `yaml:"timeout-minutes,omitempty"`
	ContinueOnError  bool                   `yaml:"continue-on-error,omitempty"`
}

// Strategy represents job matrix strategy
type Strategy struct {
	Matrix      interface{} `yaml:"matrix,omitempty"` // Can be object or array
	FailFast    *bool       `yaml:"fail-fast,omitempty"`
	MaxParallel int         `yaml:"max-parallel,omitempty"`
}

// Defaults represents default settings
type Defaults struct {
	Run *RunDefaults `yaml:"run,omitempty"`
}

// RunDefaults represents default run settings
type RunDefaults struct {
	Shell            string `yaml:"shell,omitempty"`
	WorkingDirectory string `yaml:"working-directory,omitempty"`
}

// Parser handles workflow parsing and validation
type Parser struct {
	logger interface{} // Will be replaced with proper logger interface
}

// NewParser creates a new workflow parser
func NewParser(logger interface{}) *Parser {
	return &Parser{
		logger: logger,
	}
}

// ParseFile parses a workflow file
func (p *Parser) ParseFile(filename string) (*Workflow, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow file: %w", err)
	}

	return p.Parse(data)
}

// Parse parses workflow YAML data
func (p *Parser) Parse(data []byte) (*Workflow, error) {
	var workflow Workflow
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		return nil, fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	// Validate workflow
	if err := p.validate(&workflow); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	return &workflow, nil
}

// validate performs basic workflow validation
func (p *Parser) validate(wf *Workflow) error {
	if wf.Name == "" {
		return fmt.Errorf("workflow name is required")
	}

	if len(wf.Jobs) == 0 {
		return fmt.Errorf("workflow must contain at least one job")
	}

	// Validate each job
	for jobID, job := range wf.Jobs {
		if err := p.validateJob(jobID, job); err != nil {
			return fmt.Errorf("job %s: %w", jobID, err)
		}
	}

	return nil
}

// validateJob validates a single job
func (p *Parser) validateJob(jobID string, job *Job) error {
	if job.RunsOn == nil {
		return fmt.Errorf("runs-on is required")
	}

	if len(job.Steps) == 0 {
		return fmt.Errorf("job must contain at least one step")
	}

	// Validate each step
	for i, step := range job.Steps {
		if err := p.validateStep(i, step); err != nil {
			return fmt.Errorf("step %d: %w", i, err)
		}
	}

	return nil
}

// validateStep validates a single step
func (p *Parser) validateStep(index int, step *Step) error {
	if step.Run == "" && step.Uses == "" {
		return fmt.Errorf("step must have either 'run' or 'uses'")
	}

	if step.Run != "" && step.Uses != "" {
		return fmt.Errorf("step cannot have both 'run' and 'uses'")
	}

	return nil
}

// GetJobDependencies returns the dependencies for a job
func (wf *Workflow) GetJobDependencies(jobID string) []string {
	job := wf.Jobs[jobID]
	if job == nil || job.Needs == nil {
		return nil
	}

	var deps []string
	switch needs := job.Needs.(type) {
	case string:
		deps = []string{needs}
	case []interface{}:
		for _, dep := range needs {
			if depStr, ok := dep.(string); ok {
				deps = append(deps, depStr)
			}
		}
	case []string:
		deps = needs
	}

	return deps
}

// GetRunsOn returns the runs-on value for a job
func (job *Job) GetRunsOn() []string {
	if job.RunsOn == nil {
		return []string{"ubuntu-latest"}
	}

	switch runsOn := job.RunsOn.(type) {
	case string:
		return []string{runsOn}
	case []interface{}:
		var result []string
		for _, item := range runsOn {
			if itemStr, ok := item.(string); ok {
				result = append(result, itemStr)
			}
		}
		return result
	case []string:
		return runsOn
	default:
		return []string{"ubuntu-latest"}
	}
}
