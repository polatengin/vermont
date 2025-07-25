# Vermont - Simplified GitHub Actions Runner

A lightweight, simplified GitHub Actions runner written in Go that executes workflows locally in Docker containers.

## Features

- ✅ **Simple CLI** - Single command: `vermont <workflow-file>`
- ✅ **YAML workflow parsing** - Standard GitHub Actions format
- ✅ **Container execution** - Automatic Docker runner image building
- ✅ **Multiple OS support** - Ubuntu, Debian, Alpine runners
- ✅ **Environment variables** - Configurable via JSON config
- ✅ **Temp directory isolation** - Pipeline-specific workspaces
- ✅ **Real shell command execution** - Proper bash/sh script support
- ✅ **Matrix strategy support** - Multi-dimensional build matrices
- ✅ **GitHub Actions support** - Local and remote action execution

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/polatengin/vermont.git
cd vermont

# Build the application
make build

# Or run directly without building
go run . examples/simple-test.yml
```

### Usage

#### Running a Workflow

```bash
# Run a workflow file
./bin/vermont examples/simple-test.yml

# Run with go run (no compilation needed)
go run . examples/parallel-test.yml

# Example output:
Executing workflow: Simple Test
Job: hello
  Runs on: ubuntu-latest
  Steps: 4
  Container: vermont-runner:ubuntu-latest (exists)
    Step 1: Hello World
Hello, World!
    Step 2: Show environment
Runner OS: 
GitHub Actor: 
Working Directory: /workspace
    Step 3: Create file
    Step 4: Show file content
Vermont Runner Test
Workflow completed successfully!
## Configuration

Vermont uses a simple JSON configuration file for environment variables:

```json
{
  "env": {
    "GITHUB_TOKEN": "${GITHUB_TOKEN}",
    "CI": "true",
    "GITHUB_ACTIONS": "true",
    "RUNNER_OS": "Linux",
    "GITHUB_ACTOR": "vermont-runner"
  }
}
```

Environment variables with `${VAR}` syntax will be expanded from your system environment, or fall back to `fake-<var>` values for testing.

## Supported Workflow Features

### Basic Workflow Syntax

Vermont supports standard GitHub Actions workflow syntax:

```yaml
name: Simple Test
on: push  # Single trigger event

# Multiple trigger events also supported
# on: [push, pull_request]

jobs:
  hello:
    runs-on: ubuntu-latest
    steps:
      - name: Hello World
        run: echo "Hello, World!"
      - name: Multi-line script
        run: |
          echo "Starting task"
          date
          echo "Task completed"
```

### Supported Runners

- `ubuntu-latest`, `ubuntu-22.04`, `ubuntu-20.04`
- `debian-latest`, `debian-12`, `debian-11`  
- `alpine-latest`

Vermont automatically builds Docker images for these runners from the `runners/` directory.

### Environment Variables

Vermont currently supports step-level environment variables:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Use environment
        run: echo "Token: $GITHUB_TOKEN"
        env:
          CUSTOM_VAR: "custom value"
```

**Note**: Workflow-level and job-level environment variables are not yet supported.

### Matrix Builds

Vermont supports GitHub Actions matrix strategy for multi-dimensional builds:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        version: [1.21, 1.22, 1.23]
        os: [ubuntu, macos, windows]
    steps:
      - name: Show Matrix Values
        run: echo "Running on ${{ matrix.os }} with version ${{ matrix.version }}"
```

Matrix builds automatically expand into multiple jobs (3×3=9 jobs in this example) with variable substitution.

### GitHub Actions Support

Vermont supports both local and remote GitHub Actions:

#### Local Composite Actions
```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Use local action
        uses: ./path/to/action
        with:
          name: World
          greeting: Hello
```

#### Remote Actions from GitHub
```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 1
```

Actions are automatically cloned to a `steps/` directory and executed with proper input/output handling.

### Job Dependencies (Not Yet Implemented)

While Vermont parses job dependencies, they are not yet executed in dependency order:

```yaml
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - run: echo "Running tests"
  
  build:
    runs-on: ubuntu-latest
    needs: test  # Parsed but not enforced
    steps:
      - run: echo "Building after tests"
```

Currently all jobs run in parallel regardless of `needs` declarations.

### Workflow and Job Environment Variables (Not Yet Implemented)

These environment variable levels are not yet supported:

```yaml
# Workflow-level env (not supported)
env:
  GLOBAL_VAR: "value"

jobs:
  test:
    runs-on: ubuntu-latest
    # Job-level env (not supported) 
    env:
      JOB_VAR: "value"
    steps:
      - name: Step with env
        env:
          STEP_VAR: "value"  # This works
        run: echo "Only step-level env variables work"
```
## Example Workflows

Vermont includes consolidated example workflows demonstrating all capabilities:

### 1. `basic-tests.yml`
- **Purpose**: Basic Vermont functionality testing
- **Covers**: Simple commands, environment variables, shell execution, configuration testing
- **Usage**: `go run . examples/basic-tests.yml`

### 2. `checkout-tests.yml` 
- **Purpose**: GitHub Actions checkout functionality
- **Covers**: Repository cloning, different checkout configurations, token usage
- **Usage**: `go run . examples/checkout-tests.yml`

### 3. `container-tests.yml`
- **Purpose**: Container and runner testing
- **Covers**: Different OS runners (Alpine, Ubuntu, Debian), custom images, container execution
- **Usage**: `go run . examples/container-tests.yml`

### 4. `matrix-tests.yml`
- **Purpose**: Matrix build strategies
- **Covers**: Multi-dimensional matrices, build variations, parallel matrix execution
- **Usage**: `go run . examples/matrix-tests.yml`

### 5. `dependency-tests.yml`
- **Purpose**: Job dependencies and parallel execution
- **Covers**: Job ordering, parallel jobs, dependency chains, concurrent execution
- **Usage**: `go run . examples/dependency-tests.yml`

### 6. `actions-tests.yml`
- **Purpose**: GitHub Actions marketplace integration
- **Covers**: External actions, composite actions, multiple actions in workflow
- **Usage**: `go run . examples/actions-tests.yml`

### 7. `error-tests.yml`
- **Purpose**: Error handling and edge cases
- **Covers**: Command failures, container errors, missing dependencies, circular dependencies
- **Usage**: `go run . examples/error-tests.yml`

### 8. `ci-pipeline-demo.yml`
- **Purpose**: Complete CI/CD pipeline demonstration
- **Covers**: Multi-stage pipeline, conditional deployment, environment variables, notifications
- **Usage**: `go run . examples/ci-pipeline-demo.yml`

### Local Actions
The `examples/actions/` directory contains local composite actions for testing:
- `hello-composite/` - Example composite action with inputs and steps

### Configuration Requirements
Most examples require a proper `config.json` file with:
- `GITHUB_TOKEN` for repository access
- `RUNNER_TEMP` and `RUNNER_TOOL_CACHE` for action execution
- `GITHUB_SHA`, `GITHUB_REF`, etc. for GitHub context

See `example-configs/container-config.json` for a complete configuration example.

## Development

### Quick Development

```bash
# Run without compilation
go run . examples/simple-test.yml

# Build and test
make build
./bin/vermont examples/parallel-test.yml

# Code quality checks
make fmt
make vet
make lint
```

### Container Management

Vermont automatically builds runner images when needed:

```bash
# Check built images
docker images | grep vermont-runner

# Images are built from runners/ directory
ls runners/
# Dockerfile.ubuntu-latest
# Dockerfile.debian-latest  
# Dockerfile.alpine-latest
# ...
```

## Troubleshooting

### Common Issues

**YAML Parse Errors with `needs`:**
```bash
# ❌ This fails (single string not yet supported)
needs: setup

# ✅ Use array syntax instead  
needs: [setup]
```

**Environment Variables Not Working:**
- ✅ Step-level: `env:` under each step works
- ❌ Job-level: `env:` under job doesn't work yet
- ❌ Workflow-level: top-level `env:` doesn't work yet

**Action Failures:**
- Check if action is compatible (composite/Node.js only)
- Ensure required inputs are provided in `with:` section
- Verify action repository exists and version is correct

**Container Issues:**
- Ensure Docker is running and accessible
- Check if custom runner images build successfully
- Vermont automatically falls back to ubuntu-latest for unknown runners

## Architecture

Vermont uses a simplified single-file architecture designed for direct workflow execution:

```text
┌──────────────────────────────────────────────────────────────┐
│                    Vermont - Simple CLI                     │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────┐ │
│  │ CLI Entry Point │  │ Workflow Parser │  │ Job Executor  │ │
│  │   (main.go)     │  │   (YAML)        │  │  (Sequential) │ │
│  └─────────────────┘  └─────────────────┘  └───────────────┘ │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────┐ │
│  │  Step Executor  │  │ Container Mgmt  │  │ Temp Dir Mgmt │ │
│  │ (Shell Commands)│  │   (Docker)      │  │ (Per Pipeline)│ │
│  └─────────────────┘  └─────────────────┘  └───────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

### Core Components

1. **Single CLI Interface**
   - Direct workflow execution: `vermont <workflow-file>`
   - No subcommands or complex flags
   - Automatic container management
   - Pipeline-specific temp directories

2. **Workflow Parser**
   - YAML validation and parsing using `gopkg.in/yaml.v3`
   - Flexible runs-on field handling (string or array)
   - Environment variable support
   - Basic job dependency handling

3. **Job Executor**
   - Sequential job execution
   - Container management via Docker
   - Automatic runner image building
   - Environment variable injection
   - Temp directory management

4. **Container Management**
   - Automatic image building from `runners/` directory
   - Support for multiple OS variants (Ubuntu, Debian, Alpine)
   - Image caching and reuse
   - Volume mounting for workspace directories
   - Shell detection (bash/sh)

### Design Principles

- **Minimal CLI Interface** - Single entry point with no complex commands
- **Self-Contained Execution** - Pipeline-specific directories, automatic container building
- **GitHub Actions Compatibility** - Standard YAML workflow format
- **Clean Isolation** - No external dependencies beyond Docker

### File Structure
```
vermont/
├── main.go              # Single-file implementation
├── config.json          # Environment configuration
├── examples/            # Test workflows
├── runners/             # Dockerfiles for runner images
└── Makefile             # Build and development tasks
```

### Key Data Structures
```go
type Workflow struct {
    Name string           `yaml:"name"`
    On   interface{}      `yaml:"on"`
    Jobs map[string]*Job  `yaml:"jobs"`
    // Env map[string]string `yaml:"env"` // Not yet implemented
}

type Job struct {
    RunsOn   interface{} `yaml:"runs-on"`
    Needs    []string    `yaml:"needs"`    // Parsed but not executed
    Steps    []*Step     `yaml:"steps"`
    Strategy *Strategy   `yaml:"strategy"` // Matrix support
    // Env map[string]string `yaml:"env"`  // Not yet implemented
}

type Step struct {
    Name string `yaml:"name"`
    Run  string `yaml:"run"`
    Uses string `yaml:"uses"`                    // GitHub Actions support
    With map[string]interface{} `yaml:"with"`   // Action inputs
    Env  map[string]string `yaml:"env"`         // Step-level env (working)
}
```

### Execution Flow
1. Parse command line argument (workflow file)
2. Load configuration from config.json
3. Parse YAML workflow file
4. Create pipeline temp directory
5. Execute jobs sequentially
6. Build/use container images as needed
7. Execute steps in containers
8. Clean up temp directories

## Current Feature Support

| Feature | Status | Notes |
|---------|--------|-------|
| **Basic Workflows** | ✅ Full Support | YAML parsing, job execution |
| **Container Execution** | ✅ Full Support | Ubuntu, Debian, Alpine runners |
| **Step Environment Variables** | ✅ Full Support | Per-step `env:` mapping |
| **Matrix Builds** | ✅ Full Support | Multi-dimensional with variable substitution |
| **GitHub Actions** | ✅ Partial Support | Composite and Node.js actions |
| **Local Actions** | ✅ Full Support | `./path/to/action` syntax |
| **Remote Actions** | ✅ Full Support | GitHub marketplace with versioning |
| **Action Inputs/Outputs** | ✅ Full Support | Template substitution working |
| **Job Dependencies** | ❌ Not Implemented | `needs:` parsed but ignored |
| **Workflow Environment** | ❌ Not Implemented | Top-level `env:` not supported |
| **Job Environment** | ❌ Not Implemented | Job-level `env:` not supported |
| **Conditional Execution** | ❌ Not Implemented | `if:` conditions not supported |
| **Job Outputs** | ❌ Not Implemented | Cross-job data sharing |
| **Secrets** | ❌ Not Implemented | `${{ secrets.* }}` not supported |
| **Artifacts** | ❌ Not Implemented | Upload/download not supported |
| **Services** | ❌ Not Implemented | Database containers not supported |
| **Docker Actions** | ❌ Not Implemented | Only composite/Node.js actions |

## Limitations

This simplified version focuses on core functionality:

- ✅ Basic workflow execution
- ✅ Container-based steps  
- ✅ Step-level environment variables
- ✅ Multiple OS runners
- ✅ Matrix builds with variable substitution
- ✅ GitHub Actions (composite and basic Node.js)
- ❌ **Job dependencies** (needs field parsed but not executed)
- ❌ **Workflow-level environment variables** 
- ❌ **Job-level environment variables**
- ❌ **Conditional execution** (if conditions)
- ❌ **Job outputs and step outputs**
- ❌ **Flexible needs syntax** (string vs array)
- ❌ **Complex job dependencies and parallel execution**
- ❌ **Secrets management**
- ❌ **Artifacts upload/download**
- ❌ **Docker actions**
- ❌ **Services and databases**

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Test with example workflows
5. Submit a pull request

This is a simplified implementation focused on core GitHub Actions workflow execution with container support.
