.PHONY: help build test lint lint-fix format format-check clean install-tools

# Keep in step with .github/workflows/ci.yml
GOLANGCI_LINT_VERSION := v2.13.2

# Default target
help:
	@echo "Available targets:"
	@echo "  make build         - Build the project"
	@echo "  make test          - Run tests"
	@echo "  make test-verbose  - Run tests with verbose output"
	@echo "  make lint          - Run linter (golangci-lint)"
	@echo "  make lint-fix      - Run linter and auto-fix issues"
	@echo "  make format        - Format code with goimports"
	@echo "  make format-check  - Check if code is formatted"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make install-tools - Install required tools"

# Build the project
build:
	go build -v ./...

# Run tests
test:
	go test -v -race -coverprofile=coverage.out ./...

# Run tests with verbose output
test-verbose:
	go test -v -race -coverprofile=coverage.out ./...

# Run tests and show coverage
test-coverage: test
	go tool cover -html=coverage.out

# Run linter. golangci-lint must be built with a Go at least as new as the one
# in go.mod, so install it with `make install-tools` rather than from a release.
lint:
	golangci-lint run --timeout 5m ./...

# Run linter and auto-fix
lint-fix:
	golangci-lint run --fix --timeout 5m ./...

# Format code. GOFMT comes from the active toolchain: a gofmt from an older Go
# rejects generic methods with "method must have no type parameters".
GOFMT := $(shell go env GOROOT)/bin/gofmt

format:
	$(GOFMT) -w -s .
	goimports -local github.com/mhrlife/goai-kit -w .

# Check formatting
format-check:
	@if [ -n "$$($(GOFMT) -l .)" ]; then \
		echo "The following files are not formatted:"; \
		$(GOFMT) -l .; \
		exit 1; \
	fi
	@out=$$(goimports -local github.com/mhrlife/goai-kit -l . 2>&1); \
	if [ -n "$$out" ]; then \
		echo "goimports is unhappy (misordered imports, or a goimports older"; \
		echo "than the Go in go.mod, which cannot parse generic methods):"; \
		echo "$$out"; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out

# Install required tools
install-tools:
	@echo "Installing golangci-lint (from source: a release binary built with an"
	@echo "older Go refuses a module targeting a newer one)"
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	@echo "Installing goimports..."
	go install golang.org/x/tools/cmd/goimports@latest
	@echo "All tools installed!"

# Run all checks (format + lint + test)
check: format-check lint test

# Tidy dependencies
tidy:
	go mod tidy
	go mod verify
