# Vermont GitHub Actions Runner - AI Coding Instructions

Vermont is a **local GitHub Actions runner** written in Go that executes workflows in Docker containers with full dependency management and parallel execution support.

## Architecture Overview

Vermont has evolved from single-file to a **structured package-based architecture**:
- **Entry Point**: `cmd/main.go` - CLI with `vermont <workflow-file>` interface
- **Core Packages**: `pkg/{workflow,executor,actions,container}` for modular functionality
- **Configuration**: JSON-based environment management with `${VAR}` shell expansion
- **Execution Model**: Dependency-aware job scheduling with Docker containerization

## Key Components & Data Flow

### 1. Workflow Processing (`pkg/workflow/`)
```go
// Workflow parsing with matrix expansion and validation
Workflow → Jobs → Matrix Expansion → Job Dependencies → Execution Queue
```

### 2. Job Execution (`pkg/executor/`)
```go
// Parallel execution with dependency management
JobScheduler → Dependency Validation → Parallel Execution → Container Management
```

### 3. Container Management (`pkg/container/`)
```go
// Docker-based step execution with runner images
Manager.RunStep() → Docker Pull → Container Execution → Result Collection
```

### 4. GitHub Actions Support (`pkg/actions/`)
```go
// Full GitHub Actions compatibility
Execute() → {Node.js, Docker, Composite} Action Types → Template Processing
```

## Critical Development Patterns

### 1. Container-First Execution
All steps execute in Docker containers with automatic image building:
```bash
# Auto-built from runners/Dockerfile.<runner-name>
"ubuntu-latest" → "vermont-runner:ubuntu-latest"
```

### 2. Environment Variable System
```go
// config.json with shell expansion - NO hardcoded defaults
"GITHUB_TOKEN": "${GITHUB_TOKEN}"  // Expands from environment
"GITHUB_SHA": "actual-commit-hash"  // Direct configuration values
```

### 3. Matrix Strategy Implementation
```go
// Matrix jobs expand into discrete jobs with template substitution
"test" with 2x3 matrix → "test-0", "test-1", ..., "test-5"
// Uses ${{ matrix.* }} template replacement
```

### 4. Dependency Management
```go
// Real dependency resolution with cycle detection and parallel execution
JobScheduler.validateDependencies() → Topological sort → Parallel execution
```

## Development Workflows

### Quick Testing & Debugging
```bash
# Development mode (no compilation)
go run ./cmd/main.go examples/basic-tests.yml

# Container debugging
docker images | grep vermont-runner
docker run -it vermont-runner:ubuntu-latest bash

# Dependency debugging - check job execution order
go run ./cmd/main.go examples/dependency-tests.yml -v
```

### Action Development & Testing
```bash
# Test GitHub Actions integration
go run ./cmd/main.go examples/checkout-tests.yml

# Test local composite actions
go run ./cmd/main.go examples/actions-tests.yml

# Test matrix strategies
go run ./cmd/main.go examples/matrix-tests.yml
```

## Project-Specific Conventions

### 1. Error Handling Pattern
```go
// Structured logging with context and error wrapping
logger.Error("Action execution failed", "job", jobID, "step", stepNum, "error", err)
return fmt.Errorf("step %d (action %s) failed: %w", stepNum, step.Uses, err)
```

### 2. Container Execution Pattern
```go
// Always use --network host for GitHub operations
dockerArgs := []string{"run", "--rm", "--network", "host", ...}
```

### 3. Template Processing
```go
// Multiple template contexts: matrix, inputs, step outputs
substituteMatrixVars(text, matrixVars)         // ${{ matrix.* }}
substituteActionTemplates(text, inputs, ...)   // ${{ inputs.* }}, ${{ steps.*.outputs.* }}
```

### 4. Workspace Isolation
```go
// Job-specific workspace directories
jobWorkDir := fmt.Sprintf("%s-%s", config.Runner.WorkDir, jobID)
```

## Integration Points

### GitHub Actions Compatibility
- **Action Types**: Composite, Node.js, Docker actions with full marketplace support
- **Template System**: Complete `${{ }}` syntax with context-aware substitution
- **Environment Files**: GITHUB_OUTPUT, GITHUB_STEP_SUMMARY standard support
- **Network Access**: `--network host` for repository cloning and external services

### Docker Integration
- **Image Management**: Auto-building from `runners/Dockerfile.*` with caching
- **Volume Mounting**: Workspace isolation with proper permission handling
- **Output Capture**: Combined stdout/stderr with structured logging

### Configuration System
- **Environment Expansion**: `${VAR}` syntax with fallback to shell environment
- **Structured Config**: JSON-based with validation and type safety
- **Runtime Context**: Dynamic GitHub Actions environment variable injection

## Key Architecture Decisions

### 1. Package Structure Evolution
```go
// Evolved from main.go monolith to structured packages
cmd/main.go           // CLI entry point
pkg/workflow/         // YAML parsing, matrix expansion, validation
pkg/executor/         // Job scheduling, parallel execution
pkg/actions/          // GitHub Actions execution (Node.js, Docker, Composite)
pkg/container/        // Docker management, image building
internal/config/      // Configuration management
internal/logger/      // Structured logging
```

### 2. Execution Model
- **Dependency-aware**: Real topological sorting with cycle detection
- **Parallel execution**: Configurable concurrency with semaphore control
- **Container isolation**: Every step runs in Docker for consistency
- **Error handling**: Fail-fast with detailed context logging

### 3. GitHub Actions Integration
- **Full compatibility**: Supports marketplace actions, local actions, composite actions
- **Template processing**: Complete `${{ }}` expression evaluation
- **Environment context**: Proper GitHub Actions environment variable setup

## Testing Strategy

Use consolidated examples for comprehensive testing:
```bash
# Test core functionality
go run ./cmd/main.go examples/basic-tests.yml

# Test GitHub integration (requires GITHUB_TOKEN in config.json)
go run ./cmd/main.go examples/checkout-tests.yml

# Test parallel job execution and dependencies
go run ./cmd/main.go examples/dependency-tests.yml

# Test error handling and edge cases
go run ./cmd/main.go examples/error-tests.yml

# Test complex CI pipeline scenarios
go run ./cmd/main.go examples/ci-pipeline-demo.yml
```

When modifying Vermont, always test against multiple examples to ensure compatibility across the full feature set, especially dependency resolution and container execution patterns.
