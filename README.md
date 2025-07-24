# Vermont - Lightweight GitHub Actions Runner Clone

A lightweight, self-hosted GitHub Actions runner clone written in Go.

## Features

- ✅ YAML workflow parsing and validation
- ✅ Basic step execution (shell commands)
- ✅ Job dependency management
- ✅ Environment variable support
- ✅ Container execution support
- ✅ Web interface and REST API
- ✅ Action registry and caching
- ✅ Matrix builds support

## Quick Start

### Installation

```bash
# Clone the repository
git clone https://github.com/polatengin/vermont.git
cd vermont

# Build the application
go build -o bin/vermont ./cmd/runner
```

### Usage

#### Running a Workflow

```bash
# Run a workflow file
./bin/vermont run examples/simple-test.yml

# Run with custom configuration
./bin/vermont run examples/ci-pipeline.yml -c config.json
```

#### Validating a Workflow

```bash
# Validate a workflow
./bin/vermont validate examples/simple-test.yml

# Validate with verbose output
./bin/vermont validate examples/ci-pipeline.yml -v
```

## Configuration

Vermont uses JSON configuration files. Generate a default configuration:

```bash
# Create default config (config will be created on first run)
./bin/vermont run examples/simple-test.yml
```

Example configuration:

```json
{
  "runner": {
    "workDir": "/tmp/vermont-runner",
    "maxConcurrentJobs": 2,
    "timeout": 3600,
    "labels": ["self-hosted", "vermont"]
  },
  "container": {
    "runtime": "docker",
    "dockerHost": "unix:///var/run/docker.sock",
    "networkMode": "bridge",
    "defaultImage": "ubuntu:22.04"
  },
  "storage": {
    "dataDir": "~/.vermont",
    "cacheDir": "~/.vermont/cache",
    "logsDir": "~/.vermont/logs"
  },
  "logging": {
    "level": "info",
    "format": "console",
    "file": ""
  }
}
```

## Supported Workflow Features

### Basic Syntax
- ✅ Workflow triggers (`on`)
- ✅ Jobs with `runs-on`
- ✅ Steps with `run` and `uses`
- ✅ Environment variables
- ✅ Job dependencies (`needs`)
- ✅ Conditional execution (`if`)

### Advanced Features
- ✅ Matrix builds (`strategy.matrix`)
- 🔄 Composite actions
- 🔄 Reusable workflows
- 🔄 Service containers
- 🔄 Artifacts
- 🔄 Secrets management

## Architecture

Vermont is a single binary application with two main commands:

1. **`vermont run`** - Executes workflows
2. **`vermont validate`** - Validates workflow syntax

### Core Packages

- `pkg/workflow` - YAML parsing and validation
- `pkg/executor` - Job and step execution
- `pkg/container` - Container management
- `pkg/actions` - Action registry

## Development

### Prerequisites

- Go 1.21 or later
- Docker (for container execution)

### Building

```bash
# Install dependencies
go mod download

# Build the application
make build

# Run tests
make test

# Run linter
make lint
```

### Testing

```bash
# Run unit tests
go test ./...

# Test with example workflows
./bin/vermont validate examples/simple-test.yml
./bin/vermont run examples/simple-test.yml
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test` and `make lint`
6. Submit a pull request

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Roadmap

See [design.md](design.md) for detailed architecture and implementation plans.

### Phase 1 (Current)
- [x] Basic project structure
- [x] Workflow parsing
- [x] Configuration management
- [ ] Step execution engine
- [ ] Container integration

### Phase 2
- [ ] Action registry
- [ ] Job scheduler
- [ ] Web interface
- [ ] Matrix builds

### Phase 3
- [ ] Artifact management
- [ ] Secret management
- [ ] Webhook support
- [ ] Performance optimization
