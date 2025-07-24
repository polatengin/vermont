package actions

import (
	"fmt"
	"regexp"
	"strings"
)

// TemplateProcessor handles GitHub Actions template processing
type TemplateProcessor struct {
	inputs  map[string]interface{}
	env     map[string]string
	outputs map[string]string
}

// NewTemplateProcessor creates a new template processor
func NewTemplateProcessor(inputs map[string]interface{}, env map[string]string) *TemplateProcessor {
	return &TemplateProcessor{
		inputs:  inputs,
		env:     env,
		outputs: make(map[string]string),
	}
}

// ProcessTemplate processes GitHub Actions template expressions in a string
func (tp *TemplateProcessor) ProcessTemplate(text string) string {
	// Process ${{ ... }} expressions
	re := regexp.MustCompile(`\$\{\{\s*([^}]+)\s*\}\}`)

	return re.ReplaceAllStringFunc(text, func(match string) string {
		// Extract the expression inside ${{ }}
		expression := strings.TrimSpace(match[3 : len(match)-2])

		// Process the expression
		return tp.evaluateExpression(expression)
	})
}

// evaluateExpression evaluates a template expression
func (tp *TemplateProcessor) evaluateExpression(expr string) string {
	expr = strings.TrimSpace(expr)

	// Handle inputs.xxx
	if strings.HasPrefix(expr, "inputs.") {
		inputName := expr[7:] // Remove "inputs."
		if value, exists := tp.inputs[inputName]; exists {
			return fmt.Sprintf("%v", value)
		}
		return ""
	}

	// Handle env.xxx
	if strings.HasPrefix(expr, "env.") {
		envName := expr[4:] // Remove "env."
		if value, exists := tp.env[envName]; exists {
			return value
		}
		return ""
	}

	// Handle steps.xxx.outputs.xxx
	if strings.HasPrefix(expr, "steps.") && strings.Contains(expr, ".outputs.") {
		// Extract step ID and output name
		parts := strings.Split(expr, ".")
		if len(parts) >= 4 && parts[2] == "outputs" {
			stepId := parts[1]
			outputName := parts[3]
			outputKey := fmt.Sprintf("%s.%s", stepId, outputName)
			if value, exists := tp.outputs[outputKey]; exists {
				return value
			}
		}
		return ""
	}

	// Handle runner.xxx
	if strings.HasPrefix(expr, "runner.") {
		runnerProp := expr[7:] // Remove "runner."
		switch runnerProp {
		case "os":
			return "Linux"
		case "arch":
			return "X64"
		case "name":
			return "Vermont Runner"
		case "tool-cache":
			return "/opt/hostedtoolcache"
		}
		return ""
	}

	// Handle github.xxx
	if strings.HasPrefix(expr, "github.") {
		githubProp := expr[7:] // Remove "github."
		switch githubProp {
		case "actor":
			return "vermont-runner"
		case "repository":
			return "local/repository"
		case "event_name":
			return "push"
		case "sha":
			return "abc123"
		case "ref":
			return "refs/heads/main"
		}
		return ""
	}

	// Handle hashFiles() function
	if strings.HasPrefix(expr, "hashFiles(") && strings.HasSuffix(expr, ")") {
		// For now, return a simple hash
		return "abc123hash"
	}

	// Default: return the expression as-is (or empty for unknown)
	return fmt.Sprintf("${%s}", expr)
}

// SetStepOutput sets an output for a step
func (tp *TemplateProcessor) SetStepOutput(stepId, outputName, value string) {
	outputKey := fmt.Sprintf("%s.%s", stepId, outputName)
	tp.outputs[outputKey] = value
}

// GetStepOutput gets an output for a step
func (tp *TemplateProcessor) GetStepOutput(stepId, outputName string) string {
	outputKey := fmt.Sprintf("%s.%s", stepId, outputName)
	return tp.outputs[outputKey]
}
