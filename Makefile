# Makefile for rela

# Variables
BINARY_NAME := rela
BUILD_DIR := bin
GO := go
GOLANGCI_LINT := golangci-lint
GOLANGCI_LINT_VERSION := v1.62.2

# Build flags
LDFLAGS := -s -w
GOFLAGS := -trimpath

.PHONY: all build clean test test-coverage coverage coverage-check coverage-html lint lint-fix fmt vet install-tools install-hooks help fuzz fuzz-short lint-md lint-md-fix fmt-md

# Default target
all: lint test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/rela

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf $(BUILD_DIR)
	$(GO) clean -cache -testcache

# Run tests
test:
	@echo "Running tests..."
	$(GO) test -race -cover ./...

# Run tests with verbose output
test-verbose:
	@echo "Running tests (verbose)..."
	$(GO) test -race -cover -v ./...

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	$(GO) test -race -coverprofile=coverage.out -covermode=atomic ./...

# Generate coverage report
coverage: test-coverage
	@echo "Generating coverage report..."
	$(GO) tool cover -func=coverage.out

# Check coverage thresholds
coverage-check: test-coverage
	@echo "Checking coverage thresholds..."
	@./scripts/check-coverage.sh

# Generate HTML coverage report
coverage-html: test-coverage
	@echo "Generating HTML coverage report..."
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Run fuzz tests (30 seconds each)
fuzz:
	@echo "Running fuzz tests..."
	$(GO) test -run='^$$' -fuzz='^FuzzParseDocument$$' -fuzztime=30s ./internal/markdown/
	$(GO) test -run='^$$' -fuzz='^FuzzParseEntityID$$' -fuzztime=30s ./internal/model/
	$(GO) test -run='^$$' -fuzz='^FuzzValidateID$$' -fuzztime=30s ./internal/model/
	$(GO) test -run='^$$' -fuzz='^FuzzParseRelationFilename$$' -fuzztime=30s ./internal/markdown/
	$(GO) test -run='^$$' -fuzz='^FuzzParse$$' -fuzztime=30s ./internal/metamodel/
	$(GO) test -run='^$$' -fuzz='^FuzzParseErrorQuality$$' -fuzztime=30s ./internal/metamodel/

# Run quick fuzz tests (5 seconds each)
fuzz-short:
	@echo "Running quick fuzz tests..."
	$(GO) test -run='^$$' -fuzz='^FuzzParseDocument$$' -fuzztime=5s ./internal/markdown/
	$(GO) test -run='^$$' -fuzz='^FuzzParseEntityID$$' -fuzztime=5s ./internal/model/
	$(GO) test -run='^$$' -fuzz='^FuzzValidateID$$' -fuzztime=5s ./internal/model/
	$(GO) test -run='^$$' -fuzz='^FuzzParseRelationFilename$$' -fuzztime=5s ./internal/markdown/
	$(GO) test -run='^$$' -fuzz='^FuzzParse$$' -fuzztime=5s ./internal/metamodel/
	$(GO) test -run='^$$' -fuzz='^FuzzParseErrorQuality$$' -fuzztime=5s ./internal/metamodel/

# Run linter (Go)
lint:
	@echo "Running Go linter..."
	$(GOLANGCI_LINT) run

# Lint markdown files
lint-md:
	@echo "Linting markdown files..."
	npx markdownlint-cli2 "**/*.md" "#node_modules"

# Lint and fix markdown files
lint-md-fix:
	@echo "Linting and fixing markdown files..."
	npx markdownlint-cli2 --fix "**/*.md" "#node_modules"

# Format markdown files with prettier
fmt-md:
	@echo "Formatting markdown files..."
	npx prettier --write "**/*.md" --ignore-path .gitignore

# Run linter with auto-fix
lint-fix:
	@echo "Running linter with auto-fix..."
	$(GOLANGCI_LINT) run --fix

# Run linter on specific packages
lint-pkg:
	@echo "Running linter on $(PKG)..."
	$(GOLANGCI_LINT) run $(PKG)/...

# Format code
fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...
	goimports -w -local github.com/Sourcehaven-BV/rela .

# Run go vet
vet:
	@echo "Running go vet..."
	$(GO) vet ./...

# Install development tools
install-tools:
	@echo "Installing development tools..."
	@echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION)..."
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin $(GOLANGCI_LINT_VERSION)
	@echo "Installing goimports..."
	$(GO) install golang.org/x/tools/cmd/goimports@latest
	@echo "Done!"

# Install git hooks
install-hooks:
	@echo "Installing git hooks..."
	@mkdir -p .git/hooks
	@cp scripts/pre-commit .git/hooks/pre-commit
	@chmod +x .git/hooks/pre-commit
	@echo "Git hooks installed!"

# Check if tools are installed
check-tools:
	@echo "Checking development tools..."
	@which $(GOLANGCI_LINT) > /dev/null || (echo "golangci-lint not found. Run 'make install-tools'" && exit 1)
	@echo "All tools installed!"

# Run all checks (lint + test)
check: lint lint-md test

# Run all checks and build
ci: check coverage-check build

# Tidy go modules
tidy:
	@echo "Tidying modules..."
	$(GO) mod tidy

# Download dependencies
deps:
	@echo "Downloading dependencies..."
	$(GO) mod download

# Show help
help:
	@echo "Available targets:"
	@echo "  all            - Run lint, test, and build (default)"
	@echo "  build          - Build the binary"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run tests"
	@echo "  test-verbose   - Run tests with verbose output"
	@echo "  test-coverage  - Run tests with coverage report"
	@echo "  coverage       - Generate and display coverage report"
	@echo "  coverage-check - Check coverage meets minimum thresholds"
	@echo "  coverage-html  - Generate HTML coverage report"
	@echo "  lint           - Run golangci-lint"
	@echo "  lint-fix       - Run golangci-lint with auto-fix"
	@echo "  lint-md        - Lint markdown files"
	@echo "  lint-md-fix    - Lint and fix markdown files"
	@echo "  fmt            - Format code with gofmt and goimports"
	@echo "  fmt-md         - Format markdown files with prettier"
	@echo "  vet            - Run go vet"
	@echo "  install-tools  - Install development tools (golangci-lint, goimports)"
	@echo "  install-hooks  - Install git pre-commit hooks"
	@echo "  check-tools    - Verify development tools are installed"
	@echo "  check          - Run lint, lint-md, and test"
	@echo "  ci             - Run all checks including coverage (for CI pipelines)"
	@echo "  tidy           - Run go mod tidy"
	@echo "  deps           - Download dependencies"
	@echo "  fuzz           - Run fuzz tests (30s each)"
	@echo "  fuzz-short     - Run quick fuzz tests (5s each)"
	@echo "  help           - Show this help message"
