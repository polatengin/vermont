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

.PHONY: all build clean install uninstall lint fmt vet deps help

# Default target
all: build

# Build the binary
build:
	@echo "Building vermont..."
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/vermont .

# Clean build artifacts
clean: uninstall
	@echo "Cleaning..."
	rm -rf $(BIN_DIR)
	go clean -cache

# Install to system
install: clean build
	@echo "Installing Vermont to system..."
	@if [ "$(shell id -u)" != "0" ]; then \
		echo "Installing to /usr/local/bin (may require sudo)..."; \
		sudo install -m 755 $(BIN_DIR)/vermont /usr/local/bin/vermont; \
	else \
		echo "Installing to /usr/local/bin..."; \
		install -m 755 $(BIN_DIR)/vermont /usr/local/bin/vermont; \
	fi
	@echo "Vermont installed successfully!"
	@echo "You can now run 'vermont --version' from anywhere."

# Uninstall from system
uninstall:
	@echo "Uninstalling Vermont from system..."
	@if [ "$(shell id -u)" != "0" ]; then \
		echo "Removing from /usr/local/bin (may require sudo)..."; \
		sudo rm -f /usr/local/bin/vermont; \
	else \
		echo "Removing from /usr/local/bin..."; \
		rm -f /usr/local/bin/vermont; \
	fi
	@echo "Vermont uninstalled successfully!"

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

# Help
help:
	@echo "Vermont Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  build          Build the binary"
	@echo "  clean          Clean build artifacts"
	@echo "  install        Clean, build, and install Vermont to system (/usr/local/bin)"
	@echo "  uninstall      Remove Vermont from system"
	@echo "  lint           Run linter"
	@echo "  fmt            Format code"
	@echo "  vet            Vet code"
	@echo "  deps           Download dependencies"
	@echo "  help           Show this help"
