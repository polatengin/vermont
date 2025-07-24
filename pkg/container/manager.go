package container

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/polatengin/vermont/internal/config"
	"github.com/polatengin/vermont/internal/logger"
	"github.com/polatengin/vermont/pkg/workflow"
)

// StepResult represents the result of a container step execution
type StepResult struct {
	Success  bool
	Output   string
	Error    string
	Duration time.Duration
}

// Manager handles container operations
type Manager struct {
	config *config.Config
	logger *logger.Logger
}

// NewManager creates a new container manager
func NewManager(cfg *config.Config, log *logger.Logger) *Manager {
	return &Manager{
		config: cfg,
		logger: log,
	}
}

// RunStep executes a step in a container
func (m *Manager) RunStep(ctx context.Context, step *workflow.Step, image string, env map[string]string, workDir string) (StepResult, error) {
	start := time.Now()

	// Pull image if needed
	if err := m.PullImage(ctx, image); err != nil {
		return StepResult{
			Success:  false,
			Error:    fmt.Sprintf("Failed to pull image %s: %v", image, err),
			Duration: time.Since(start),
		}, err
	}

	// Prepare the container command
	containerID := fmt.Sprintf("vermont-step-%d", time.Now().UnixNano())

	// Build docker run command
	args := []string{
		"run",
		"--rm",
		"--name", containerID,
		"--workdir", "/workspace",
	}

	// Add environment variables
	for key, value := range env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add step-specific environment variables
	for key, value := range step.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Mount workspace if workDir is provided
	if workDir != "" {
		args = append(args, "-v", fmt.Sprintf("%s:/workspace", workDir))
	}

	// Add the image
	args = append(args, image)

	// Add the command to execute
	if step.Run != "" {
		// Determine shell based on image
		shell := m.getShellForImage(image)
		args = append(args, shell, "-c", step.Run)
	} else {
		// Default command for the image
		shell := m.getShellForImage(image)
		args = append(args, shell)
	}

	// Execute the container
	cmd := exec.CommandContext(ctx, "docker", args...)

	m.logger.Debug("Running container command",
		"image", image,
		"containerID", containerID,
		"command", step.Run)

	// Capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		m.logger.Error("Container execution failed",
			"image", image,
			"containerID", containerID,
			"error", err,
			"output", outputStr)

		return StepResult{
			Success:  false,
			Output:   outputStr,
			Error:    fmt.Sprintf("Container execution failed: %v", err),
			Duration: time.Since(start),
		}, err
	}

	m.logger.Debug("Container execution completed",
		"image", image,
		"containerID", containerID,
		"duration", time.Since(start))

	return StepResult{
		Success:  true,
		Output:   outputStr,
		Error:    "",
		Duration: time.Since(start),
	}, nil
}

// PullImage pulls a container image
func (m *Manager) PullImage(ctx context.Context, image string) error {
	// Check if image already exists locally
	checkCmd := exec.CommandContext(ctx, "docker", "image", "inspect", image)
	if err := checkCmd.Run(); err == nil {
		// Image exists locally
		m.logger.Debug("Image already exists locally", "image", image)
		return nil
	}

	m.logger.Info("Pulling container image", "image", image)

	// Pull the image
	cmd := exec.CommandContext(ctx, "docker", "pull", image)

	// Stream the output to show pull progress
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start docker pull: %w", err)
	}

	// Read output in real-time
	go m.streamOutput(stdout, "PULL")
	go m.streamOutput(stderr, "PULL-ERR")

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("failed to pull image %s: %w", image, err)
	}

	m.logger.Info("Image pulled successfully", "image", image)
	return nil
}

// streamOutput streams container output line by line
func (m *Manager) streamOutput(reader io.Reader, prefix string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			m.logger.Debug("Container output", "prefix", prefix, "line", line)
		}
	}
}

// Cleanup cleans up container resources
func (m *Manager) Cleanup(ctx context.Context) error {
	m.logger.Debug("Cleaning up container resources")

	// Remove any dangling containers with vermont prefix
	cmd := exec.CommandContext(ctx, "docker", "container", "prune", "-f", "--filter", "label=vermont=true")
	if err := cmd.Run(); err != nil {
		m.logger.Warn("Failed to cleanup containers", "error", err)
	}

	return nil
}

// IsDockerAvailable checks if Docker is available
func (m *Manager) IsDockerAvailable(ctx context.Context) bool {
	cmd := exec.CommandContext(ctx, "docker", "version")
	err := cmd.Run()
	available := err == nil

	if !available {
		m.logger.Warn("Docker is not available")
	}

	return available
}

// GetDefaultImage returns the default container image based on runs-on
func (m *Manager) GetDefaultImage(runsOn []string) string {
	if len(runsOn) == 0 {
		return m.config.Container.DefaultImage
	}

	// Map GitHub Actions runner labels to container images
	imageMap := map[string]string{
		"ubuntu-latest": "ubuntu:22.04",
		"ubuntu-22.04":  "ubuntu:22.04",
		"ubuntu-20.04":  "ubuntu:20.04",
		"ubuntu-18.04":  "ubuntu:18.04",
		"debian-latest": "debian:12",
		"debian-12":     "debian:12",
		"debian-11":     "debian:11",
		"alpine-latest": "alpine:latest",
		"alpine":        "alpine:latest",
		"centos-latest": "centos:8",
		"centos-8":      "centos:8",
		"centos-7":      "centos:7",
	}

	for _, runner := range runsOn {
		if image, exists := imageMap[runner]; exists {
			return image
		}
	}

	return m.config.Container.DefaultImage
}

// getShellForImage returns the appropriate shell for the given image
func (m *Manager) getShellForImage(image string) string {
	// Alpine and other minimal images typically use sh
	if strings.Contains(image, "alpine") {
		return "sh"
	}
	
	// Most other images have bash
	return "bash"
}
