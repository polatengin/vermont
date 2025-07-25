package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the Vermont runner configuration
type Config struct {
	// Runner configuration
	Runner RunnerConfig `json:"runner"`

	// Container configuration
	Container ContainerConfig `json:"container"`

	// Storage configuration
	Storage StorageConfig `json:"storage"`

	// Logging configuration
	Logging LoggingConfig `json:"logging"`

	// Actions configuration
	Actions ActionsConfig `json:"actions"`

	// Environment variables configuration
	Env map[string]string `json:"env,omitempty"`
}

// RunnerConfig contains runner-specific settings
type RunnerConfig struct {
	// WorkDir is the working directory for workflow execution
	WorkDir string `json:"workDir"`

	// MaxConcurrentJobs is the maximum number of concurrent jobs
	MaxConcurrentJobs int `json:"maxConcurrentJobs"`

	// Timeout is the default timeout for job execution (in seconds)
	Timeout int `json:"timeout"`

	// Labels are the runner labels for job matching
	Labels []string `json:"labels"`
}

// ContainerConfig contains container runtime settings
type ContainerConfig struct {
	// Runtime specifies the container runtime (docker, podman, etc.)
	Runtime string `json:"runtime"`

	// DockerHost is the Docker daemon socket
	DockerHost string `json:"dockerHost"`

	// NetworkMode specifies the container network mode
	NetworkMode string `json:"networkMode"`

	// DefaultImage is the default container image for jobs
	DefaultImage string `json:"defaultImage"`
}

// StorageConfig contains storage settings
type StorageConfig struct {
	// DataDir is the directory for persistent data
	DataDir string `json:"dataDir"`

	// CacheDir is the directory for cached actions and artifacts
	CacheDir string `json:"cacheDir"`

	// LogsDir is the directory for execution logs
	LogsDir string `json:"logsDir"`
}

// LoggingConfig contains logging settings
type LoggingConfig struct {
	// Level is the logging level (debug, info, warn, error)
	Level string `json:"level"`

	// Format is the log format (json, console)
	Format string `json:"format"`

	// File is the log file path (empty for stdout)
	File string `json:"file"`
}

// ActionsConfig contains GitHub Actions settings
type ActionsConfig struct {
	// Registry is the base URL for the actions registry
	Registry string `json:"registry"`

	// CacheEnabled enables action caching
	CacheEnabled bool `json:"cacheEnabled"`

	// CacheTTL is the cache time-to-live in hours (0 = no expiration)
	CacheTTL int `json:"cacheTtl"`

	// AllowedOrgs is a list of allowed GitHub organizations (empty = all)
	AllowedOrgs []string `json:"allowedOrgs,omitempty"`

	// NodejsVersion is the default Node.js version for node actions
	NodejsVersion string `json:"nodejsVersion"`
}

// Default returns a default configuration
func Default() *Config {
	// Use current directory instead of home directory
	currentDir, _ := os.Getwd()
	dataDir := filepath.Join(currentDir, ".vermont")

	return &Config{
		Runner: RunnerConfig{
			WorkDir:           "/tmp/vermont-runner",
			MaxConcurrentJobs: 2,
			Timeout:           3600, // 1 hour
			Labels:            []string{"self-hosted", "vermont"},
		},
		Container: ContainerConfig{
			Runtime:      "docker",
			DockerHost:   "unix:///var/run/docker.sock",
			NetworkMode:  "bridge",
			DefaultImage: "vermont-runner:latest",
		},
		Storage: StorageConfig{
			DataDir:  dataDir,
			CacheDir: filepath.Join(dataDir, "cache"),
			LogsDir:  filepath.Join(dataDir, "logs"),
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "console",
			File:   "",
		},
		Actions: ActionsConfig{
			Registry:      "https://github.com",
			CacheEnabled:  true,
			CacheTTL:      24,         // 24 hours
			AllowedOrgs:   []string{}, // Empty = allow all
			NodejsVersion: "20",
		},
		Env: make(map[string]string),
	}
}

// Load loads configuration from file or returns default config
func Load(configFile string) (*Config, error) {
	if configFile == "" {
		return Default(), nil
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Initialize env map if nil
	if config.Env == nil {
		config.Env = make(map[string]string)
	}

	// Apply environment variables from config
	if err := config.applyEnvironmentVariables(); err != nil {
		return nil, fmt.Errorf("failed to apply environment variables: %w", err)
	}

	// Ensure directories exist
	if err := config.ensureDirectories(); err != nil {
		return nil, fmt.Errorf("failed to create directories: %w", err)
	}

	return &config, nil
}

// Save saves the configuration to a file
func (c *Config) Save(configFile string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// applyEnvironmentVariables sets environment variables from the config
func (c *Config) applyEnvironmentVariables() error {
	for key, value := range c.Env {
		// Support environment variable expansion (e.g., ${GITHUB_TOKEN})
		expandedValue := os.ExpandEnv(value)
		if err := os.Setenv(key, expandedValue); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}
	return nil
}

// GetEnvironmentVariables returns a copy of all environment variables from config
func (c *Config) GetEnvironmentVariables() map[string]string {
	env := make(map[string]string)
	for key, value := range c.Env {
		// Support environment variable expansion
		env[key] = os.ExpandEnv(value)
	}
	return env
}

// SetEnvironmentVariable sets an environment variable in the config
func (c *Config) SetEnvironmentVariable(key, value string) {
	if c.Env == nil {
		c.Env = make(map[string]string)
	}
	c.Env[key] = value
}

// ensureDirectories creates necessary directories
func (c *Config) ensureDirectories() error {
	dirs := []string{
		c.Storage.DataDir,
		c.Storage.CacheDir,
		c.Storage.LogsDir,
		c.Runner.WorkDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}
