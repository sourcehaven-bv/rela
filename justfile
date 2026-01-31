# rela project justfile

# Variables
build_dir := "bin"
golangci_lint_version := "v1.62.2"

# Default recipe
default: lint test build

# ── Build ──

# Build the CLI binary
build-cli:
    @echo "Building rela CLI..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o {{build_dir}}/rela ./cmd/rela

# Build the data entry server
build-server:
    @echo "Building rela-server..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o {{build_dir}}/rela-server ./cmd/rela-server

# Build the desktop app
build-desktop:
    @echo "Building rela-desktop..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=1 CGO_LDFLAGS="-framework UniformTypeIdentifiers" go build -tags desktop,production -trimpath -ldflags "-s -w" -o {{build_dir}}/rela-desktop ./cmd/rela-desktop

# Build all binaries
build: build-cli build-server build-desktop

# Clean build artifacts
clean:
    @echo "Cleaning..."
    rm -rf {{build_dir}}
    go clean -cache -testcache

# ── Test ──

# Run tests with race detection
test:
    @echo "Running tests..."
    go test -race -cover ./...

# Run tests with verbose output
test-verbose:
    @echo "Running tests (verbose)..."
    go test -race -cover -v ./...

# Run tests with coverage profile
test-coverage:
    @echo "Running tests with coverage..."
    go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Generate and display coverage report
coverage: test-coverage
    @echo "Generating coverage report..."
    go tool cover -func=coverage.out

# Check coverage meets minimum thresholds (uses go-test-coverage with ratchet baseline)
coverage-check: test-coverage
    @echo "Checking coverage thresholds..."
    go-test-coverage --config=.testcoverage.yml

# Generate HTML coverage report
coverage-html: test-coverage
    @echo "Generating HTML coverage report..."
    go tool cover -html=coverage.out -o coverage.html
    @echo "Coverage report generated: coverage.html"

# Run fuzz tests (30 seconds each)
fuzz:
    @echo "Running fuzz tests..."
    go test -run='^$$' -fuzz='^FuzzParseDocument$$' -fuzztime=30s ./internal/markdown/
    go test -run='^$$' -fuzz='^FuzzParseEntityID$$' -fuzztime=30s ./internal/model/
    go test -run='^$$' -fuzz='^FuzzValidateID$$' -fuzztime=30s ./internal/model/
    go test -run='^$$' -fuzz='^FuzzParseRelationFilename$$' -fuzztime=30s ./internal/markdown/

# Run quick fuzz tests (5 seconds each)
fuzz-short:
    @echo "Running quick fuzz tests..."
    go test -run='^$$' -fuzz='^FuzzParseDocument$$' -fuzztime=5s ./internal/markdown/
    go test -run='^$$' -fuzz='^FuzzParseEntityID$$' -fuzztime=5s ./internal/model/
    go test -run='^$$' -fuzz='^FuzzValidateID$$' -fuzztime=5s ./internal/model/
    go test -run='^$$' -fuzz='^FuzzParseRelationFilename$$' -fuzztime=5s ./internal/markdown/

# ── Lint & Format ──

# Run Go linter
lint:
    @echo "Running Go linter..."
    golangci-lint run

# Run linter with auto-fix
lint-fix:
    @echo "Running linter with auto-fix..."
    golangci-lint run --fix

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

# Format Go code
fmt:
    @echo "Formatting code..."
    go fmt ./...
    goimports -w -local github.com/Sourcehaven-BV/rela .

# Run go vet
vet:
    @echo "Running go vet..."
    go vet ./...

# ── CI & Checks ──

# Run all checks (lint + lint-md + test)
check: lint lint-md test

# Generate docs from rela entities via mdcomp
docs: build-cli
    @echo "Generating documentation..."
    @./scripts/generate-docs.sh

# Check that committed docs are up to date with entities
docs-check: docs
    @echo "Checking docs are up to date..."
    git diff --exit-code docs/ README.md || \
        (echo "" && echo "ERROR: docs/ or README.md is out of date." && \
         echo "Run 'just docs' and commit the changes." && exit 1)
    @echo "✓ Docs are up to date."

# Run full CI pipeline (check + coverage + build + docs)
ci: check coverage-check build docs-check

# ── Dependencies & Tools ──

# Install development tools
install-tools:
    @echo "Installing development tools..."
    @echo "Installing golangci-lint {{golangci_lint_version}}..."
    @curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin {{golangci_lint_version}}
    @echo "Installing goimports..."
    go install golang.org/x/tools/cmd/goimports@latest
    @echo "Installing go-test-coverage..."
    go install github.com/vladopajic/go-test-coverage/v2@latest
    @echo "Done!"

# Install git hooks
install-hooks:
    @echo "Installing git hooks..."
    @mkdir -p .git/hooks
    @cp scripts/pre-commit .git/hooks/pre-commit
    @chmod +x .git/hooks/pre-commit
    @cp scripts/pre-push .git/hooks/pre-push
    @chmod +x .git/hooks/pre-push
    @echo "Git hooks installed (pre-commit + pre-push)!"

# Tidy go modules
tidy:
    @echo "Tidying modules..."
    go mod tidy

# Download dependencies
deps:
    @echo "Downloading dependencies..."
    go mod download

# ── Vendor ──

# Vendor JS/CSS dependencies (commit the results)
vendor-js:
    @echo "Vendoring JS/CSS dependencies..."
    @mkdir -p internal/dataentry/static
    curl -sfL -o internal/dataentry/static/htmx.min.js         "https://unpkg.com/htmx.org@2.0.4"
    curl -sfL -o internal/dataentry/static/easymde.min.js      "https://unpkg.com/easymde@2.18.0/dist/easymde.min.js"
    curl -sfL -o internal/dataentry/static/easymde.min.css     "https://unpkg.com/easymde@2.18.0/dist/easymde.min.css"
    curl -sfL -o internal/dataentry/static/slimselect.min.js   "https://unpkg.com/slim-select@2.9.2/dist/slimselect.min.js"
    curl -sfL -o internal/dataentry/static/slimselect.css      "https://unpkg.com/slim-select@2.9.2/dist/slimselect.css"
    curl -sfL -o internal/dataentry/static/tagify.min.js       "https://unpkg.com/@yaireo/tagify@4.31.3/dist/tagify.min.js"
    curl -sfL -o internal/dataentry/static/tagify.css          "https://unpkg.com/@yaireo/tagify@4.31.3/dist/tagify.css"
    @echo "Done! Review changes with 'git diff' and commit."

# ── Dev Server ──

# Run the data entry server for development (ticketing example)
[no-exit-message]
dev project="prototypes/data-entry/project" port="8080":
    go run ./cmd/rela-server -project {{project}} -port {{port}}

# Run the catalog example
[no-exit-message]
dev-catalog port="8282":
    go run ./cmd/rela-server -project prototypes/data-entry/catalog -port {{port}}
