.PHONY: help build test lint lint-fix format format-check clean install-tools

# Source files
SRCS := $(shell find . -name '*.go' -not -path "./vendor/*")

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

# Run linter
lint:
	golangci-lint run

# Run linter and auto-fix
lint-fix:
	golangci-lint run --fix

# Format code
format:
	@echo "Running golines"
	@golines --ignore-generated --base-formatter gofmt -m 120 -w $(SRCS)
	@echo "Running gofumpt"
	@gofumpt -w $(SRCS)
	@echo "Running gci"
	@gci write --skip-generated -s standard -s default -s "prefix(git.divar.cloud/divar/search/post-list)" .
	goimports -w .
	gofmt -w -s .

# Check formatting
format-check:
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "The following files are not formatted:"; \
		gofmt -l .; \
		exit 1; \
	fi

# Clean build artifacts
clean:
	go clean
	rm -f coverage.out

# Install required tools
install-tools:
	@echo "Installing golangci-lint..."
	@which golangci-lint > /dev/null || curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin
	@echo "Installing goimports..."
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	@echo "All tools installed!"

# Run all checks (format + lint + test)
check: format-check lint test

# Tidy dependencies
tidy:
	go mod tidy
	go mod verify
