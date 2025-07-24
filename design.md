# Vermont - Lightweight GitHub Actions Runner Clone

## Project Overview

Vermont is a lightweight, self-hosted GitHub Actions runner clone written in Go. It's designed to execute YAML workflows with support for basic GitHub Actions features, making it perfect for offline testing, on-premises deployments, or learning CI/CD concepts.

## Architecture Overview

```text
┌──────────────────────────────────────────────────────────────┐
│                        Vermont Runner                        │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────┐ │
│  │   CLI Interface │  │   Web Interface │  │  API Server   │ │
│  └─────────────────┘  └─────────────────┘  └───────────────┘ │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────┐ │
│  │ Workflow Parser │  │ Job Scheduler   │  │  Event System │ │
│  └─────────────────┘  └─────────────────┘  └───────────────┘ │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────┐ │
│  │  Step Executor  │  │ Container Mgmt  │  │  Log Manager  │ │
│  └─────────────────┘  └─────────────────┘  └───────────────┘ │
├──────────────────────────────────────────────────────────────┤
│  ┌─────────────────┐  ┌─────────────────┐  ┌───────────────┐ │
│  │ Action Registry │  │ Secret Manager  │  │ Artifact Mgmt │ │
│  └─────────────────┘  └─────────────────┘  └───────────────┘ │
└──────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. Workflow Parser

- **Purpose**: Parse GitHub Actions YAML workflows
- **Features**:
  - YAML validation and parsing
  - Workflow syntax validation
  - Dependency graph generation
  - Environment variable interpolation
- **Go Packages**: `gopkg.in/yaml.v3`, custom AST builder

### 2. Job Scheduler

- **Purpose**: Orchestrate job execution with proper dependency management
- **Features**:
  - Concurrent job execution using goroutines
  - Dependency resolution (needs, if conditions)
  - Resource allocation and limits
  - Job queuing and prioritization
- **Go Patterns**: Worker pools, channels, context cancellation

### 3. Step Executor

- **Purpose**: Execute individual workflow steps
- **Features**:
  - Shell command execution
  - Action execution (uses directive)
  - Environment variable management
  - Working directory management
  - Timeout handling
- **Go Packages**: `os/exec`, `context`, custom process management

### 4. Container Management

- **Purpose**: Handle containerized step execution
- **Features**:
  - Docker integration
  - Container lifecycle management
  - Volume mounting
  - Network isolation
  - Resource constraints
- **Integration**: Docker API, runc (future)

### 5. Action Registry

- **Purpose**: Manage and cache GitHub Actions
- **Features**:
  - Action downloading and caching
  - Composite action support
  - Local action execution
  - Version management
- **Storage**: Local filesystem cache

### 6. Event System

- **Purpose**: Handle workflow triggers and events
- **Features**:
  - Webhook handling
  - Manual triggers
  - Scheduled execution (cron)
  - Event filtering and routing
- **Go Patterns**: Event-driven architecture, pub/sub

## Supported Workflow Features

### Basic Workflow Syntax

```yaml
name: CI Pipeline
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Run tests
        run: go test ./...
        env:
          GO_VERSION: "1.21"
```

### Supported Features (MVP)

- ✅ Basic workflow triggers (push, pull_request, manual)
- ✅ Job definitions with runs-on
- ✅ Step execution (run commands)
- ✅ Action usage (uses directive)
- ✅ Environment variables
- ✅ Job dependencies (needs)
- ✅ Conditional execution (if)
- ✅ Matrix builds (basic)

### Advanced Features (Future)

- 🔄 Composite actions
- 🔄 Reusable workflows
- 🔄 Service containers
- 🔄 Job outputs and artifacts
- 🔄 Secrets management
- 🔄 Self-hosted runner registration

## Project Structure

```
vermont/
├── cmd/
│   └── runner/          # Main application
├── pkg/
│   ├── workflow/        # Workflow parsing and validation
│   ├── executor/        # Step and job execution
│   ├── container/       # Container management
│   └── actions/         # Action registry and management
├── internal/
│   ├── config/          # Configuration management
│   ├── logger/          # Structured logging
│   └── utils/           # Common utilities
├── examples/            # Example workflows
└── docs/                # Documentation
```

## Key Go Patterns and Technologies

### Concurrency Model

```go
type JobScheduler struct {
    workers    int
    jobQueue   chan *Job
    workerPool chan chan *Job
    quit       chan bool
}

func (js *JobScheduler) Start() {
    for i := 0; i < js.workers; i++ {
        worker := NewWorker(js.workerPool)
        worker.Start()
    }
    go js.dispatch()
}
```

### Workflow Execution Pipeline

```go
type Pipeline struct {
    parser    *WorkflowParser
    scheduler *JobScheduler
    executor  *StepExecutor
    logger    *Logger
}

func (p *Pipeline) Execute(ctx context.Context, workflow *Workflow) error {
    jobs, err := p.parser.Parse(workflow)
    if err != nil {
        return err
    }
    
    return p.scheduler.Schedule(ctx, jobs)
}
```

### Container Integration

```go
type ContainerExecutor struct {
    client docker.Client
    config *ContainerConfig
}

func (ce *ContainerExecutor) RunStep(step *Step) error {
    container, err := ce.createContainer(step)
    if err != nil {
        return err
    }
    defer ce.cleanup(container)
    
    return ce.execute(container, step.Command)
}
```

## Data Models

### Workflow Structure

```go
type Workflow struct {
    Name     string            `yaml:"name"`
    On       []string          `yaml:"on"`
    Jobs     map[string]*Job   `yaml:"jobs"`
    Env      map[string]string `yaml:"env,omitempty"`
}

type Job struct {
    Name      string            `yaml:"name,omitempty"`
    RunsOn    string            `yaml:"runs-on"`
    Needs     []string          `yaml:"needs,omitempty"`
    If        string            `yaml:"if,omitempty"`
    Steps     []*Step           `yaml:"steps"`
    Env       map[string]string `yaml:"env,omitempty"`
    Strategy  *Strategy         `yaml:"strategy,omitempty"`
}

type Step struct {
    Name string            `yaml:"name,omitempty"`
    Run  string            `yaml:"run,omitempty"`
    Uses string            `yaml:"uses,omitempty"`
    With map[string]string `yaml:"with,omitempty"`
    Env  map[string]string `yaml:"env,omitempty"`
    If   string            `yaml:"if,omitempty"`
}
```

## Implementation Phases

### Phase 1: Core Foundation

1. Project setup and basic CLI
2. YAML workflow parsing
3. Basic step execution (shell commands)
4. Logging and error handling

### Phase 2: Job Orchestration

5. Job scheduler with dependency management
6. Concurrent execution with goroutines
7. Environment variable handling
8. Conditional execution support

### Phase 3: Action Support

9. Action registry and caching
10. GitHub Actions downloading
11. Composite action support
12. Local action execution

### Phase 4: Container Integration

13. Docker integration
14. Container lifecycle management
15. Volume and network management
16. Resource constraints

### Phase 5: Advanced Features

17. Web interface for workflow monitoring
18. REST API for external integration
19. Matrix build support
20. Artifact management

### Phase 6: Production Ready
21. Security hardening
22. Performance optimization
23. Comprehensive testing
24. Documentation and examples

## Technical Challenges & Solutions

### Challenge 1: Workflow Dependency Resolution

**Problem**: Managing complex job dependencies and execution order
**Solution**: Implement a topological sort algorithm with cycle detection

### Challenge 2: Concurrent Execution Management

**Problem**: Balancing parallelism with resource constraints
**Solution**: Worker pool pattern with configurable concurrency limits

### Challenge 3: Container Security

**Problem**: Ensuring secure container execution
**Solution**: User namespaces, resource limits, and security contexts

### Challenge 4: Action Compatibility

**Problem**: Supporting diverse GitHub Actions
**Solution**: Standardized action interface with plugin architecture

## Performance Considerations

- **Memory Management**: Streaming YAML parsing for large workflows
- **CPU Utilization**: Optimal worker pool sizing based on system resources
- **I/O Optimization**: Parallel action downloading and caching
- **Container Overhead**: Container reuse for multiple steps when possible

## Security Features

- **Sandbox Execution**: All steps run in isolated containers
- **Secret Management**: Encrypted secret storage with minimal exposure
- **Network Isolation**: Controlled network access for containers
- **File System Protection**: Read-only mounts and temporary directories

## Monitoring and Observability

- **Structured Logging**: JSON-formatted logs with correlation IDs
- **Metrics Collection**: Prometheus-compatible metrics
- **Tracing**: OpenTelemetry integration for distributed tracing
- **Health Checks**: Comprehensive health monitoring

## Future Enhancements

1. **Kubernetes Integration**: Native K8s job execution
2. **Multi-Runner Coordination**: Distributed runner management
3. **Advanced Caching**: Intelligent build cache management
4. **Plugin System**: Custom step executors and integrations
5. **Cloud Integration**: AWS/GCP/Azure native integrations

## Success Metrics

- Successfully execute 90% of common GitHub Actions workflows
- Sub-second startup time for simple workflows
- Memory usage under 100MB for typical workflows
- Support for at least 50 concurrent jobs
- 99.9% uptime for long-running deployments

This design showcases advanced Go programming concepts, distributed systems knowledge, and deep understanding of CI/CD principles that will definitely impress your teammates!
