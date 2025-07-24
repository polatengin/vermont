# Vermont Development Guide

## Running Without Compilation

You don't need to compile Vermont every time during development. Use `go run` to execute directly from source:

### Direct Go Commands

```bash
# Show help
go run ./cmd/runner --help

# Show version  
go run ./cmd/runner --version

# Validate a workflow
go run ./cmd/runner validate examples/simple-test.yml

# Run a workflow
go run ./cmd/runner run examples/simple-test.yml

# Validate any YAML file
go run ./cmd/runner validate path/to/your/workflow.yml

# Run any YAML file
go run ./cmd/runner run path/to/your/workflow.yml
```

### Makefile Development Targets

For convenience, use these Makefile targets:

```bash
# Quick validate (no compilation) - uses default file
make dev-validate

# Quick validate with custom file
make dev-validate FILE=examples/ci-pipeline.yml
make dev-validate FILE=path/to/your/workflow.yml

# Quick run (no compilation) - uses default file
make dev-run

# Quick run with custom file
make dev-run FILE=examples/matrix-build.yml
make dev-run FILE=path/to/your/workflow.yml

# Execute any command (no compilation)
make dev-exec ARGS="validate examples/ci-pipeline.yml"
make dev-exec ARGS="--help"
make dev-exec ARGS="--version"

# Traditional compiled approach
make dev-server
```

### Development Workflow

1. **Make code changes**
2. **Test immediately**: `go run ./cmd/runner validate examples/simple-test.yml`
3. **Run workflow**: `go run ./cmd/runner run examples/simple-test.yml`
4. **No build step needed** during development!

### Performance Notes

- `go run` compiles in memory and runs immediately
- Slightly slower than pre-compiled binary (1-2 seconds)
- Perfect for development and testing
- No need to manage binary artifacts during development

### Production vs Development

| Approach | Use Case | Speed | Convenience |
|----------|----------|-------|-------------|
| `go run` | Development | Slower startup | High |
| `make build` | Production | Fast startup | Medium |
| `make dev-exec` | Development | Slower startup | Highest |

## Tips

- Use `go run` during active development
- Use compiled binary for CI/CD and production
- The `dev-exec` target is most flexible for testing different commands
- All approaches use the same source code - no functional differences
