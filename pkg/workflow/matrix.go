package workflow

import (
	"fmt"
	"strings"
)

// MatrixCombination represents a single combination of matrix variables
type MatrixCombination map[string]interface{}

// ExpandMatrixJobs expands jobs with matrix strategies into multiple jobs
func (wf *Workflow) ExpandMatrixJobs() error {
	expandedJobs := make(map[string]*Job)

	for jobID, job := range wf.Jobs {
		if job.Strategy == nil || job.Strategy.Matrix == nil {
			// No matrix strategy, keep the job as is
			expandedJobs[jobID] = job
			continue
		}

		// Expand matrix job
		matrixJobs, err := wf.expandMatrixJob(jobID, job)
		if err != nil {
			return fmt.Errorf("failed to expand matrix job %s: %w", jobID, err)
		}

		// Add all expanded jobs
		for expandedJobID, expandedJob := range matrixJobs {
			expandedJobs[expandedJobID] = expandedJob
		}
	}

	// Replace jobs with expanded versions
	wf.Jobs = expandedJobs
	return nil
}

// expandMatrixJob expands a single job with matrix strategy
func (wf *Workflow) expandMatrixJob(jobID string, job *Job) (map[string]*Job, error) {
	combinations, err := wf.generateMatrixCombinations(job.Strategy.Matrix)
	if err != nil {
		return nil, err
	}

	expandedJobs := make(map[string]*Job)

	for _, combination := range combinations {
		// Create expanded job ID
		expandedJobID := fmt.Sprintf("%s (%s)", jobID, wf.formatMatrixCombination(combination))

		// Clone the original job
		expandedJob := wf.cloneJobWithMatrixValues(job, combination)

		expandedJobs[expandedJobID] = expandedJob
	}

	return expandedJobs, nil
}

// generateMatrixCombinations generates all combinations from matrix definition
func (wf *Workflow) generateMatrixCombinations(matrix interface{}) ([]MatrixCombination, error) {
	switch m := matrix.(type) {
	case map[string]interface{}:
		return wf.generateCombinationsFromMap(m)
	case map[interface{}]interface{}:
		// Convert to string keys
		stringMap := make(map[string]interface{})
		for k, v := range m {
			if keyStr, ok := k.(string); ok {
				stringMap[keyStr] = v
			}
		}
		return wf.generateCombinationsFromMap(stringMap)
	default:
		return nil, fmt.Errorf("unsupported matrix type: %T", matrix)
	}
}

// generateCombinationsFromMap generates combinations from a map of arrays
func (wf *Workflow) generateCombinationsFromMap(matrixMap map[string]interface{}) ([]MatrixCombination, error) {
	if len(matrixMap) == 0 {
		return []MatrixCombination{{}}, nil
	}

	// Convert matrix values to slices
	matrixVars := make(map[string][]interface{})
	for key, value := range matrixMap {
		switch v := value.(type) {
		case []interface{}:
			matrixVars[key] = v
		case []string:
			// Convert []string to []interface{}
			interfaceSlice := make([]interface{}, len(v))
			for i, s := range v {
				interfaceSlice[i] = s
			}
			matrixVars[key] = interfaceSlice
		default:
			// Single value, treat as single-element array
			matrixVars[key] = []interface{}{value}
		}
	}

	// Generate all combinations
	var combinations []MatrixCombination
	wf.generateCombinationsRecursive(matrixVars, MatrixCombination{}, &combinations)

	return combinations, nil
}

// generateCombinationsRecursive recursively generates all combinations
func (wf *Workflow) generateCombinationsRecursive(matrixVars map[string][]interface{}, current MatrixCombination, results *[]MatrixCombination) {
	if len(current) == len(matrixVars) {
		// Complete combination, add to results
		combination := make(MatrixCombination)
		for k, v := range current {
			combination[k] = v
		}
		*results = append(*results, combination)
		return
	}

	// Find next variable to process
	for varName, values := range matrixVars {
		if _, exists := current[varName]; !exists {
			// Process this variable
			for _, value := range values {
				current[varName] = value
				wf.generateCombinationsRecursive(matrixVars, current, results)
				delete(current, varName)
			}
			break
		}
	}
}

// cloneJobWithMatrixValues creates a copy of the job with matrix values applied
func (wf *Workflow) cloneJobWithMatrixValues(job *Job, combination MatrixCombination) *Job {
	// Deep clone the job
	cloned := &Job{
		Name:           job.Name,
		RunsOn:         job.RunsOn,
		Needs:          job.Needs,
		If:             job.If,
		Env:            make(map[string]string),
		Container:      job.Container,
		Services:       job.Services,
		Outputs:        job.Outputs,
		TimeoutMinutes: job.TimeoutMinutes,
		Defaults:       job.Defaults,
		Strategy:       nil, // Remove strategy from expanded job
	}

	// Copy environment variables and apply matrix substitution
	for k, v := range job.Env {
		cloned.Env[k] = wf.substituteMatrixVariables(v, combination)
	}

	// Clone and process steps
	for _, step := range job.Steps {
		clonedStep := &Step{
			ID:               step.ID,
			Name:             wf.substituteMatrixVariables(step.Name, combination),
			Run:              wf.substituteMatrixVariables(step.Run, combination),
			Uses:             wf.substituteMatrixVariables(step.Uses, combination),
			With:             make(map[string]interface{}),
			Env:              make(map[string]string),
			If:               wf.substituteMatrixVariables(step.If, combination),
			Shell:            step.Shell,
			WorkingDirectory: step.WorkingDirectory,
			TimeoutMinutes:   step.TimeoutMinutes,
			ContinueOnError:  step.ContinueOnError,
		}

		// Copy and process step.With
		for k, v := range step.With {
			if strVal, ok := v.(string); ok {
				clonedStep.With[k] = wf.substituteMatrixVariables(strVal, combination)
			} else {
				clonedStep.With[k] = v
			}
		}

		// Copy and process step environment
		for k, v := range step.Env {
			clonedStep.Env[k] = wf.substituteMatrixVariables(v, combination)
		}

		cloned.Steps = append(cloned.Steps, clonedStep)
	}

	return cloned
}

// substituteMatrixVariables replaces matrix variable references in strings
func (wf *Workflow) substituteMatrixVariables(input string, combination MatrixCombination) string {
	result := input
	for varName, value := range combination {
		placeholder := fmt.Sprintf("${{ matrix.%s }}", varName)
		valueStr := fmt.Sprintf("%v", value)
		result = strings.ReplaceAll(result, placeholder, valueStr)
	}
	return result
}

// formatMatrixCombination formats a matrix combination for display
func (wf *Workflow) formatMatrixCombination(combination MatrixCombination) string {
	var parts []string
	for key, value := range combination {
		parts = append(parts, fmt.Sprintf("%s: %v", key, value))
	}
	return strings.Join(parts, ", ")
}
