# Build variables
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")

# Go variables
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

# Build flags
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

# Directories
BIN_DIR = bin
CMD_DIR = cmd

.PHONY: all build build-verbose clean test lint fmt vet deps dev-deps help docker-test

# Default target
all: build

# Build the binary
build: build-runner

# Build the binary with verbose output
build-verbose: 
	@echo "Building Vermont..."
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Date: $(DATE)"
	@echo "Cleaning previous builds..."
	@rm -rf $(BIN_DIR)
	@echo "Creating bin directory..."
	@mkdir -p $(BIN_DIR)
	@$(MAKE) build-runner
	@echo "Build completed successfully!"
	@echo "Binary created in ./$(BIN_DIR)/"
	@echo ""
	@echo "Binary information:"
	@ls -la $(BIN_DIR)/
	@echo ""
	@echo "Testing version:"
	@./$(BIN_DIR)/vermont --version

# Build the runner binary
build-runner:
	@echo "Building vermont..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/vermont ./$(CMD_DIR)/runner

# Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	go clean -cache

# Run tests
test:
	@echo "Running Vermont tests..."
	@echo "Running unit tests..."
	go test -v -race -coverprofile=coverage.out ./...
	@echo "Generating coverage report..."
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage summary:"
	go tool cover -func=coverage.out
	@echo "Test completed successfully!"
	@echo "Coverage report generated: coverage.html"

# Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

# Vet code
vet:
	@echo "Vetting code..."
	go vet ./...

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	go mod download
	go mod tidy

# Install development tools
dev-deps:
	@echo "Installing development tools..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run example workflows
test-examples: build-runner
	@echo "Testing example workflows..."
	@echo "Validating simple-test.yml..."
	./$(BIN_DIR)/vermont validate examples/simple-test.yml
	@echo "Running simple-test.yml..."
	./$(BIN_DIR)/vermont run examples/simple-test.yml
	@echo "Validating ci-pipeline.yml..."
	./$(BIN_DIR)/vermont validate examples/ci-pipeline.yml

# Cross-compile for multiple platforms
build-cross:
	@echo "Cross-compiling..."
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ $$os = "windows" ]; then ext=".exe"; else ext=""; fi; \
			echo "Building for $$os/$$arch..."; \
			GOOS=$$os GOARCH=$$arch go build -ldflags "$(LDFLAGS)" \
				-o $(BIN_DIR)/vermont-$$os-$$arch$$ext ./$(CMD_DIR)/runner; \
		done \
	done

# Create release archives
release: build-cross
	@echo "Creating release archives..."
	@mkdir -p releases
	@for os in linux darwin windows; do \
		for arch in amd64 arm64; do \
			if [ $$os = "windows" ]; then ext=".exe"; else ext=""; fi; \
			archive="releases/vermont-$(VERSION)-$$os-$$arch"; \
			if [ $$os = "windows" ]; then \
				zip -r $$archive.zip $(BIN_DIR)/vermont-$$os-$$arch$$ext README.md LICENSE examples/; \
			else \
				tar -czf $$archive.tar.gz $(BIN_DIR)/vermont-$$os-$$arch$$ext README.md LICENSE examples/; \
			fi; \
		done \
	done

# Docker build
docker-build:
	@echo "Building Docker image..."
	docker build -t vermont:$(VERSION) .
	docker tag vermont:$(VERSION) vermont:latest

# Docker test - example of running with dynamic commands
docker-test: docker-build
	@echo "Testing Docker image with dynamic commands..."
	@echo "Testing version command:"
	docker run --rm vermont:$(VERSION) --version
	@echo "Testing help command:"
	docker run --rm vermont:$(VERSION) --help

# Development server
dev-server: build-runner
	@echo "Starting development runner..."
	./$(BIN_DIR)/vermont run examples/simple-test.yml

# Help
help:
	@echo "Vermont Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  build-verbose  Build the binary with verbose output"
	@echo "  build-runner   Build runner binary"
	@echo "  clean          Clean build artifacts"
	@echo "  test           Run tests with coverage"
	@echo "  lint           Run linter"
	@echo "  fmt            Format code"
	@echo "  vet            Vet code"
	@echo "  deps           Download dependencies"
	@echo "  dev-deps       Install development tools"
	@echo "  test-examples  Test example workflows"
	@echo "  build-cross    Cross-compile for multiple platforms"
	@echo "  release        Create release archives"
	@echo "  docker-build   Build Docker image"
	@echo "  docker-test    Test Docker image with dynamic commands"
	@echo "  dev-server     Run development test"
	@echo "  help           Show this help"
