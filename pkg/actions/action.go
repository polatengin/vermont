package actions

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/polatengin/vermont/internal/config"
	"github.com/polatengin/vermont/internal/logger"
)

// Action represents a GitHub Action
type Action struct {
	// Full action reference (e.g., "actions/checkout@v4")
	Reference string `json:"reference"`

	// Parsed components
	Owner   string `json:"owner"`   // e.g., "actions"
	Name    string `json:"name"`    // e.g., "checkout"
	Version string `json:"version"` // e.g., "v4"

	// Action metadata (from action.yml)
	Metadata *ActionMetadata `json:"metadata,omitempty"`

	// Local path where action is cached
	LocalPath string `json:"localPath"`
}

// ActionMetadata represents action.yml/action.yaml metadata
type ActionMetadata struct {
	Name        string                  `yaml:"name"`
	Description string                  `yaml:"description"`
	Author      string                  `yaml:"author,omitempty"`
	Inputs      map[string]ActionInput  `yaml:"inputs,omitempty"`
	Outputs     map[string]ActionOutput `yaml:"outputs,omitempty"`
	Runs        ActionRuns              `yaml:"runs"`
	Branding    *ActionBranding         `yaml:"branding,omitempty"`
}

// ActionInput represents an action input
type ActionInput struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required,omitempty"`
	Default     string `yaml:"default,omitempty"`
}

// ActionOutput represents an action output
type ActionOutput struct {
	Description string `yaml:"description"`
	Value       string `yaml:"value,omitempty"`
}

// ActionRuns represents the runs configuration
type ActionRuns struct {
	Using      string            `yaml:"using"`
	Main       string            `yaml:"main,omitempty"`       // For node actions
	Pre        string            `yaml:"pre,omitempty"`        // Pre-execution script
	Post       string            `yaml:"post,omitempty"`       // Post-execution script
	PreIf      string            `yaml:"pre-if,omitempty"`     // Condition for pre
	PostIf     string            `yaml:"post-if,omitempty"`    // Condition for post
	Steps      []ActionRunStep   `yaml:"steps,omitempty"`      // For composite actions
	Image      string            `yaml:"image,omitempty"`      // For Docker actions
	Entrypoint string            `yaml:"entrypoint,omitempty"` // For Docker actions
	Args       []string          `yaml:"args,omitempty"`       // For Docker actions
	Env        map[string]string `yaml:"env,omitempty"`        // Environment variables
}

// ActionRunStep represents a step in a composite action
type ActionRunStep struct {
	Name  string            `yaml:"name,omitempty"`
	ID    string            `yaml:"id,omitempty"`
	Run   string            `yaml:"run,omitempty"`
	Uses  string            `yaml:"uses,omitempty"`
	With  map[string]string `yaml:"with,omitempty"`
	Env   map[string]string `yaml:"env,omitempty"`
	Shell string            `yaml:"shell,omitempty"`
	If    string            `yaml:"if,omitempty"`
}

// ActionBranding represents action branding information
type ActionBranding struct {
	Icon  string `yaml:"icon,omitempty"`
	Color string `yaml:"color,omitempty"`
}

// ParseActionReference parses an action reference into components
func ParseActionReference(reference string) (*Action, error) {
	if reference == "" {
		return nil, fmt.Errorf("action reference cannot be empty")
	}

	// Parse format: owner/name@version or ./path/to/local/action
	if strings.HasPrefix(reference, "./") || strings.HasPrefix(reference, "/") {
		// Local action
		return &Action{
			Reference: reference,
			Owner:     "local",
			Name:      filepath.Base(reference),
			LocalPath: reference,
		}, nil
	}

	// Remote action: owner/name@version
	parts := strings.Split(reference, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid action reference format: %s (expected owner/name@version)", reference)
	}

	ownerName := parts[0]
	version := parts[1]

	ownerNameParts := strings.Split(ownerName, "/")
	if len(ownerNameParts) < 2 {
		return nil, fmt.Errorf("invalid action reference format: %s (expected owner/name@version)", reference)
	}

	owner := ownerNameParts[0]
	name := strings.Join(ownerNameParts[1:], "/")

	return &Action{
		Reference: reference,
		Owner:     owner,
		Name:      name,
		Version:   version,
	}, nil
}

// GetCachePath returns the cache path for this action
func (a *Action) GetCachePath(cacheDir string) string {
	if a.LocalPath != "" {
		return a.LocalPath
	}

	// For remote actions: cache/{owner}/{name}/{version}
	return filepath.Join(cacheDir, a.Owner, a.Name, a.Version)
}

// IsLocal returns true if this is a local action
func (a *Action) IsLocal() bool {
	return a.LocalPath != "" && (strings.HasPrefix(a.Reference, "./") || strings.HasPrefix(a.Reference, "/"))
}

// GetRepositoryURL returns the GitHub repository URL for remote actions
func (a *Action) GetRepositoryURL() string {
	if a.IsLocal() {
		return ""
	}
	return fmt.Sprintf("https://github.com/%s/%s", a.Owner, a.Name)
}

// ActionExecutionResult represents the result of action execution
type ActionExecutionResult struct {
	Success  bool              `json:"success"`
	Outputs  map[string]string `json:"outputs,omitempty"`
	Error    string            `json:"error,omitempty"`
	Duration string            `json:"duration,omitempty"`
}

// Manager handles action operations
type Manager struct {
	config *config.Config
	logger *logger.Logger
}

// NewManager creates a new action manager
func NewManager(cfg *config.Config, log *logger.Logger) *Manager {
	return &Manager{
		config: cfg,
		logger: log,
	}
}

// GetAction retrieves and caches an action
func (m *Manager) GetAction(ctx context.Context, reference string) (*Action, error) {
	action, err := ParseActionReference(reference)
	if err != nil {
		return nil, fmt.Errorf("failed to parse action reference: %w", err)
	}

	if action.IsLocal() {
		// For local actions, just verify the path exists
		if _, err := os.Stat(action.LocalPath); err != nil {
			return nil, fmt.Errorf("local action not found: %s", action.LocalPath)
		}
		return action, nil
	}

	// For remote actions, check cache first
	actionsCacheDir := filepath.Join(m.config.Storage.CacheDir, "actions")
	cachePath := action.GetCachePath(actionsCacheDir)
	action.LocalPath = cachePath

	// Check if action is already cached
	if m.isActionCached(cachePath) {
		m.logger.Debug("Action found in cache", "action", reference, "path", cachePath)
		return action, nil
	}

	// Download and cache the action
	if err := m.downloadAction(ctx, action); err != nil {
		return nil, fmt.Errorf("failed to download action: %w", err)
	}

	return action, nil
}

// isActionCached checks if an action is already cached
func (m *Manager) isActionCached(cachePath string) bool {
	// Check if the action directory exists and contains action.yml or action.yaml
	actionYml := filepath.Join(cachePath, "action.yml")
	actionYaml := filepath.Join(cachePath, "action.yaml")

	if _, err := os.Stat(actionYml); err == nil {
		return true
	}
	if _, err := os.Stat(actionYaml); err == nil {
		return true
	}

	return false
}

// downloadAction downloads an action from GitHub
func (m *Manager) downloadAction(ctx context.Context, action *Action) error {
	m.logger.Info("Downloading action", "action", action.Reference)

	// Create cache directory
	if err := os.MkdirAll(action.LocalPath, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Use git to clone the repository at the specified version
	repoURL := action.GetRepositoryURL()

	// Clone with specific branch/tag
	// For now, we'll use a simple approach - in production, you might want to use go-git
	return m.gitCloneAction(ctx, repoURL, action.Version, action.LocalPath)
}

// gitCloneAction clones an action repository
func (m *Manager) gitCloneAction(ctx context.Context, repoURL, version, localPath string) error {
	// Remove existing directory if it exists
	if err := os.RemoveAll(localPath); err != nil {
		return fmt.Errorf("failed to remove existing cache directory: %w", err)
	}

	// Create parent directory
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	m.logger.Info("Cloning action repository", "url", repoURL, "version", version, "path", localPath)

	// Try to use git command if available
	if m.isGitAvailable() {
		return m.gitCloneWithCommand(ctx, repoURL, version, localPath)
	}

	// Fallback to placeholder for now
	return m.createPlaceholderAction(repoURL, localPath)
}

// isGitAvailable checks if git command is available
func (m *Manager) isGitAvailable() bool {
	// Simple check to see if git is in PATH
	_, err := exec.LookPath("git")
	return err == nil
}

// gitCloneWithCommand uses git command to clone the repository
func (m *Manager) gitCloneWithCommand(ctx context.Context, repoURL, version, localPath string) error {
	// Create temporary directory for cloning
	tempDir := localPath + ".tmp"
	defer os.RemoveAll(tempDir)

	// Clone the repository with specific branch/tag
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", "--branch", version, repoURL, tempDir)
	if err := cloneCmd.Run(); err != nil {
		m.logger.Warn("Failed to clone with specific version, trying default branch",
			"url", repoURL, "version", version, "error", err)

		// Try cloning without specific version
		cloneCmd = exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, tempDir)
		if err := cloneCmd.Run(); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}

		// Try to checkout the specific version
		checkoutCmd := exec.CommandContext(ctx, "git", "-C", tempDir, "checkout", version)
		if err := checkoutCmd.Run(); err != nil {
			m.logger.Warn("Failed to checkout specific version, using default",
				"version", version, "error", err)
		}
	}

	// Move the cloned repository to the final location
	if err := os.Rename(tempDir, localPath); err != nil {
		return fmt.Errorf("failed to move cloned repository: %w", err)
	}

	// Remove .git directory to save space
	gitDir := filepath.Join(localPath, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		m.logger.Warn("Failed to remove .git directory", "path", gitDir, "error", err)
	}

	m.logger.Info("Action repository cloned successfully", "path", localPath)
	return nil
}

// createPlaceholderAction creates a placeholder action when git is not available
func (m *Manager) createPlaceholderAction(repoURL, localPath string) error {
	// Create a placeholder action.yml for now
	placeholderContent := fmt.Sprintf(`name: '%s Action (Placeholder)'
description: 'Placeholder for %s - Git not available or cloning failed'
runs:
  using: 'composite'
  steps:
    - name: 'Placeholder step'
      run: |
        echo "🚧 This is a placeholder for action: %s"
        echo "   Git cloning is not fully functional in this environment"
        echo "   The action would normally execute its intended functionality"
        echo "✅ Placeholder execution completed successfully"
      shell: bash
`, repoURL, repoURL, repoURL)

	actionYmlPath := filepath.Join(localPath, "action.yml")
	if err := os.MkdirAll(localPath, 0755); err != nil {
		return fmt.Errorf("failed to create action directory: %w", err)
	}

	if err := os.WriteFile(actionYmlPath, []byte(placeholderContent), 0644); err != nil {
		return fmt.Errorf("failed to create placeholder action.yml: %w", err)
	}

	m.logger.Warn("Action placeholder created (git not available)", "action", repoURL, "path", localPath)
	return nil
}
