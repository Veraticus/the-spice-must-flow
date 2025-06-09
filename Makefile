# the-spice-must-flow Makefile

BINARY_NAME=spice
MAIN_PACKAGE=./cmd/spice
GO=go
GOTEST=$(GO) test
GOVET=$(GO) vet
GOFMT=gofmt

# Build variables
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS=-ldflags "-X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

# Platform detection
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)

.PHONY: all build clean test test-verbose test-coverage test-integration bench fmt lint vet run help
.PHONY: cover fix quick setup-hooks install-tools

# Default target
all: fmt test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	$(GO) build $(LDFLAGS) -o $(BINARY_NAME) $(MAIN_PACKAGE)

# Build for specific platforms
build-linux:
	@echo "Building for Linux..."
	GOOS=linux GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 $(MAIN_PACKAGE)

build-darwin:
	@echo "Building for macOS..."
	GOOS=darwin GOARCH=amd64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 $(MAIN_PACKAGE)
	GOOS=darwin GOARCH=arm64 $(GO) build $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 $(MAIN_PACKAGE)

build-all: build-linux build-darwin

# Clean build artifacts
clean:
	@echo "Cleaning..."
	$(GO) clean
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*

# Run tests
test:
	@echo "Running tests..."
	$(GOTEST) ./...

# Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	$(GOTEST) -race ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	$(GOTEST) -v ./...

# Run tests with coverage
test-coverage:
	@echo "Running tests with coverage..."
	$(GOTEST) -cover ./...

# Run integration tests
test-integration:
	@echo "Running integration tests..."
	$(GOTEST) -tags=integration ./...

# Run benchmarks
bench:
	@echo "Running benchmarks..."
	$(GOTEST) -bench=. ./...

# Format code
fmt:
	@echo "Formatting code..."
	$(GOFMT) -s -w .

# Run linter
lint:
	@bash scripts/fix.sh

# Run go vet
vet:
	@echo "Running go vet..."
	$(GOVET) ./...

# Run the application
run: build
	./$(BINARY_NAME) $(ARGS)

# Install the binary
install: build
	@echo "Installing $(BINARY_NAME)..."
	$(GO) install $(MAIN_PACKAGE)

# Update dependencies
deps:
	@echo "Updating dependencies..."
	$(GO) mod download
	$(GO) mod tidy

# Development workflow - format, vet, and test
check: fmt vet test

# Test cross-platform builds
test-builds:
	@echo "Testing cross-platform builds..."
	@echo "=== Testing binary builds ==="
	@echo "Building for darwin/amd64..." && GOOS=darwin GOARCH=amd64 $(GO) build -o /dev/null $(MAIN_PACKAGE)
	@echo "Building for darwin/arm64..." && GOOS=darwin GOARCH=arm64 $(GO) build -o /dev/null $(MAIN_PACKAGE)
	@echo "Building for linux/amd64..." && GOOS=linux GOARCH=amd64 $(GO) build -o /dev/null $(MAIN_PACKAGE)
	@echo "Building for linux/arm64..." && GOOS=linux GOARCH=arm64 $(GO) build -o /dev/null $(MAIN_PACKAGE)
	@echo "Building for linux/386..." && GOOS=linux GOARCH=386 $(GO) build -o /dev/null $(MAIN_PACKAGE)
	@echo "=== Testing test compilation ==="
	@echo "Compiling tests for darwin/amd64..." && GOOS=darwin GOARCH=amd64 $(GO) test -c ./internal/... && rm -f *.test
	@echo "Compiling tests for darwin/arm64..." && GOOS=darwin GOARCH=arm64 $(GO) test -c ./internal/... && rm -f *.test
	@echo "Compiling tests for linux/amd64..." && GOOS=linux GOARCH=amd64 $(GO) test -c ./internal/... && rm -f *.test
	@echo "Compiling tests for linux/arm64..." && GOOS=linux GOARCH=arm64 $(GO) test -c ./internal/... && rm -f *.test
	@echo "Compiling tests for linux/386..." && GOOS=linux GOARCH=386 $(GO) test -c ./internal/... && rm -f *.test
	@echo "All platform builds and test compilations succeeded!"

# Run all tests and checks (comprehensive)
test-all: fmt vet lint test test-race test-integration test-builds
	@echo "All tests passed!"

# CI workflow - all checks
ci: deps fmt vet lint test test-race test-coverage test-builds

# Development helpers

# Generate coverage report with HTML output
cover:
	@echo "Generating coverage report..."
	$(GOTEST) -race -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Auto-fix issues
fix:
	@echo "Auto-fixing issues..."
	@bash scripts/fix.sh

# Quick check (format and test)
quick: fmt test

# Setup git hooks
setup-hooks:
	@echo "Setting up git hooks..."
	@bash scripts/setup-hooks.sh

# Install required development tools
install-tools:
	@echo "Installing development tools..."
	@bash scripts/install-tools.sh

# Help
help:
	@echo "the-spice-must-flow Makefile targets:"
	@echo ""
	@echo "Core targets:"
	@echo "  make build          - Build the binary"
	@echo "  make build-all      - Build for all platforms"
	@echo "  make clean          - Remove build artifacts"
	@echo "  make test           - Run tests"
	@echo "  make test-race      - Run tests with race detection"
	@echo "  make test-verbose   - Run tests with verbose output"
	@echo "  make test-coverage  - Run tests with coverage"
	@echo "  make test-integration - Run integration tests"
	@echo "  make test-all       - Run all tests and checks (comprehensive)"
	@echo "  make bench          - Run benchmarks"
	@echo ""
	@echo "Code quality:"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Run linter"
	@echo "  make vet            - Run go vet"
	@echo "  make check          - Run fmt, vet, and test"
	@echo "  make fix            - Auto-fix formatting and other issues"
	@echo ""
	@echo "Development helpers:"
	@echo "  make run            - Build and run the application"
	@echo "  make install        - Install the binary"
	@echo "  make deps           - Update dependencies"
	@echo "  make cover          - Generate coverage report"
	@echo "  make quick          - Format and test (for development)"
	@echo "  make setup-hooks    - Setup git hooks"
	@echo "  make install-tools  - Install required development tools"
	@echo ""
	@echo "CI/Release:"
	@echo "  make ci             - Run full CI workflow"
	@echo "  make update-nix     - Update all Nix hashes to current HEAD (use ARGS=-f to force)"
	@echo ""
	@echo "  make help           - Show this help message"
