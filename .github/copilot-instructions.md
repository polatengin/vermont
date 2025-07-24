# Vermont - AI Coding Agent Instructions

## Project Overview
Vermont is a lightweight, Go-based GitHub Actions runner clone that executes GitHub Actions workflows locally or in containers. It's designed to be a self-contained alternative to GitHub's hosted runners with focus on performance, compatibility, and ease of use.

## Architecture & Core Components

### 🏗️ **Main Components**
- **`cmd/runner/main.go`** - CLI interface with `vermont run` and `vermont validate` commands
- **`pkg/workflow/`** - YAML workflow parsing, validation, and data structures 
- **`pkg/executor/`** - Core execution engine with job scheduling and dependency management
- **`pkg/container/`** - Docker container management with automatic image building
- **`pkg/actions/`** - GitHub Actions marketplace integration and execution
- **`internal/config/`** - Configuration management for runner, container, and action settings
- **`internal/logger/`** - Structured logging throughout the application
- **`runners/`** - Dockerfiles for various OS variants (Ubuntu, Debian, Alpine, CentOS)

### 🔄 **Execution Flow**
```
CLI → Workflow Parser → Job Scheduler → Dependency Validation → Parallel Execution → Step Executor → Container/Host Execution
```

### 🧬 **Key Data Structures**
- **`Workflow`** - Top-level workflow with jobs, triggers, and global settings
- **`Job`** - Individual job with steps, dependencies (`needs`), and execution context
- **`Step`** - Single execution unit with `run` commands or `uses` actions
- **`JobScheduler`** - Manages job dependencies, parallel execution, and state tracking
- **`JobState`** - Tracks individual job status (Pending → Ready → Running → Completed/Failed)

## 🚀 **Key Features & Implementation**

### Job Dependency Management & Parallel Execution
- **Dependency Resolution**: Topological sort with circular dependency detection
- **Parallel Execution**: Goroutine-based concurrent job execution with semaphore limits
- **State Management**: Job state tracking with dependency completion monitoring
- **Error Handling**: Deadlock detection and proper error propagation

### GitHub Actions Marketplace Integration
- **Action Management**: Download, cache, and version actions from GitHub repositories
- **Action Types**: Composite actions, Node.js actions (placeholder), Docker actions (placeholder)
- **Template Processing**: Handle `${{ inputs.name }}` expressions and GitHub Actions syntax
- **Environment Setup**: Proper INPUT_* environment variable mapping for actions

### Container Execution
- **Automatic Building**: Just-in-time building of Vermont runner images from `runners/` directory
- **Image Management**: Intelligent image detection, pulling, and building
- **Multi-OS Support**: Ubuntu, Debian, Alpine, CentOS variants with full development toolchains
- **Shell Detection**: Automatic shell selection (bash for Ubuntu/Debian, sh for Alpine)

### Step Execution Engine
- **Command Execution**: Shell command execution with proper error handling
- **Environment Variables**: Job-level, step-level, and system environment inheritance
- **Container Integration**: Seamless switching between host and container execution
- **Output Management**: Real-time output streaming and structured logging

## 🛠️ **Development Patterns & Conventions**

### Go Patterns Used
- **Worker Pool Pattern**: Job scheduler with configurable concurrency limits
- **Context Propagation**: Proper context usage for cancellation and timeouts
- **Structured Logging**: Consistent logging with key-value pairs across all components
- **Dependency Injection**: Clean separation of concerns with interface-based design
- **Error Wrapping**: Proper error context with `fmt.Errorf` and `%w` verb

### Project Structure Conventions
```
pkg/          - Public APIs and core functionality
internal/     - Private packages (config, logger, utils)
cmd/          - CLI applications and main entry points
runners/      - Container dockerfiles organized by OS variant
examples/     - Comprehensive workflow examples for testing
```

### Configuration Management
- **JSON Configuration**: Structured config with runner, container, actions, logging sections
- **Environment Override**: Runtime configuration through environment variables
- **Default Values**: Sensible defaults for development and production use

### Testing Strategy
- **Example Workflows**: Comprehensive test workflows in `examples/` directory
- **Integration Testing**: End-to-end workflow execution testing
- **Error Scenarios**: Dedicated error handling and edge case testing
- **Container Testing**: Multi-OS container execution validation

## 🔧 **Development Guidelines**

### When Adding New Features
1. **Follow the Component Pattern**: Each major feature should have its own package under `pkg/`
2. **Use Structured Logging**: Always log with context using the logger with key-value pairs
3. **Handle Errors Properly**: Wrap errors with context, never ignore error returns
4. **Write Integration Tests**: Add example workflows to test new functionality
5. **Update Documentation**: Keep README.md and this file updated with new features

### Code Quality Standards
- **Linting**: Must pass `make lint` with zero issues (errcheck, staticcheck, go vet)
- **Error Handling**: All error returns must be checked and handled appropriately
- **Context Usage**: Use context.Context for cancellation and timeout handling
- **Interface Design**: Prefer interfaces for testability and loose coupling

### Container Integration Guidelines
- **Image Naming**: Follow `vermont-runner-<os>-<version>` pattern for custom images
- **Dockerfile Location**: All runner dockerfiles in `runners/` directory
- **Automatic Building**: Images should build automatically when referenced
- **Shell Compatibility**: Test with both bash and sh shells for cross-OS compatibility

### GitHub Actions Integration
- **Action Caching**: Cache downloaded actions in `~/.vermont/actions/` directory
- **Template Processing**: Handle all GitHub Actions expressions and syntax
- **Input Validation**: Validate required inputs and provide meaningful error messages
- **Environment Setup**: Properly map inputs to INPUT_* environment variables

## 🎯 **Common Development Tasks**

### Adding a New Workflow Feature
1. Update `pkg/workflow/workflow.go` with new YAML structures
2. Extend parser validation in `workflow.go`
3. Update executor logic in `pkg/executor/executor.go`
4. Add example workflow in `examples/`
5. Test with `make dev-exec ARGS="run examples/your-test.yml"`

### Adding Container Support for New OS
1. Create `runners/Dockerfile.<os>-<version>` with development toolchain
2. Update `pkg/container/manager.go` dockerfile mapping
3. Test automatic building with workflow using new OS

### Debugging Execution Issues
- Use `make dev-exec ARGS="run examples/your-workflow.yml -v"` for verbose logging
- Check container logs for container execution issues
- Verify action downloads in `~/.vermont/actions/` cache directory
- Use structured logging to trace execution flow

### Performance Optimization
- Adjust `MaxConcurrentJobs` in configuration for parallel execution tuning
- Use container reuse strategies for multiple steps
- Implement action caching optimizations
- Profile goroutine usage for job scheduling bottlenecks

## 🚨 **Common Pitfalls & Solutions**

### Error Handling
- **Never ignore errors**: Always check and handle error returns
- **Provide context**: Use `fmt.Errorf("operation failed: %w", err)` for error wrapping
- **Log before returning**: Log errors with context before returning them

### Container Execution
- **Docker availability**: Always check `IsDockerAvailable()` before container operations
- **Image building**: Expect automatic building for Vermont runner images
- **Shell differences**: Test with both bash (Ubuntu/Debian) and sh (Alpine) shells

### Job Dependencies
- **Circular dependencies**: Validate dependency graphs to prevent deadlocks
- **Missing jobs**: Ensure all `needs` references point to existing jobs
- **Parallel limits**: Configure appropriate `MaxConcurrentJobs` for system resources

### GitHub Actions Integration
- **Action compatibility**: Focus on composite actions first, Node.js/Docker are placeholders
- **Input handling**: Convert between YAML `with` and environment `INPUT_*` variables
- **Template processing**: Handle all `${{ }}` expressions in action inputs and commands

## 📚 **Reference Information**

### Key Configuration Files
- **`example-configs/host-config.json`** - Host execution configuration
- **`example-configs/container-config.json`** - Container execution configuration
- **`Makefile`** - Development tasks and build targets

### Important Example Workflows
- **`examples/simple-test.yml`** - Basic command execution
- **`examples/dependency-test.yml`** - Job dependency testing
- **`examples/parallel-test.yml`** - Parallel execution testing
- **`examples/actions-demo.yml`** - GitHub Actions marketplace integration
- **`examples/container-test.yml`** - Container execution testing

### Build & Development
- **`make build`** - Build the Vermont binary
- **`make dev-exec ARGS="run <workflow>"`** - Execute workflow in development
- **`make lint`** - Run code quality checks
- **`make test`** - Run test suite

This documentation should guide AI agents to understand Vermont's architecture, maintain code quality, and extend functionality effectively while following established patterns and conventions.
