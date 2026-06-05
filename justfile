# rela project justfile

# Variables
build_dir := "bin"
golangci_lint_version := "v1.62.2"
go_packages := "$(go list ./... | grep -v /frontend/node_modules/)"

# Default recipe
default: lint test build

# ── Build ──

# Build the CLI binary
build-cli:
    @echo "Building rela CLI..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o {{build_dir}}/rela ./cmd/rela

# Build the data entry server (includes Vue frontend)
build-server: build-frontend
    @echo "Building rela-server..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o {{build_dir}}/rela-server ./cmd/rela-server

# Build rela-server embedding the E2E (development-mode) frontend, so
# DEV-guarded test hooks are available to the E2E suite (issue #890).
build-server-e2e: build-frontend-e2e
    @echo "Building rela-server (E2E frontend)..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o {{build_dir}}/rela-server ./cmd/rela-server

# Build the desktop app
build-desktop: build-frontend
    @echo "Building rela-desktop..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=1 CGO_LDFLAGS="-framework UniformTypeIdentifiers" go build -tags desktop,production -trimpath -ldflags "-s -w" -o {{build_dir}}/rela-desktop ./cmd/rela-desktop

# Build the desktop app with debug/devtools support for E2E testing
build-desktop-debug: build-frontend
    @echo "Building rela-desktop (debug)..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=1 CGO_LDFLAGS="-framework UniformTypeIdentifiers" go build -tags desktop -o {{build_dir}}/rela-desktop ./cmd/rela-desktop

# Build the PostgreSQL-backed CLI binary (rela-postgres)
build-cli-postgres:
    @echo "Building rela-postgres CLI..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -tags postgres -trimpath -ldflags "-s -w" -o {{build_dir}}/rela-postgres ./cmd/rela

# Build the PostgreSQL-backed data entry server (rela-server-postgres)
build-server-postgres: build-frontend
    @echo "Building rela-server-postgres..."
    @mkdir -p {{build_dir}}
    CGO_ENABLED=0 go build -tags postgres -trimpath -ldflags "-s -w" -o {{build_dir}}/rela-server-postgres ./cmd/rela-server

# Build all binaries
build: build-cli build-server build-desktop

# Build the postgres-tagged binaries (FS binaries unaffected)
build-postgres: build-cli-postgres build-server-postgres

# Install CLI to ~/bin
install: build-cli build-server
    @echo "Installing rela and rela-server to ~/bin..."
    @mkdir -p ~/bin
    @install {{build_dir}}/rela ~/bin/rela
    @install {{build_dir}}/rela-server ~/bin/rela-server
    @echo "Done! Make sure ~/bin is in your PATH."

# Clean build artifacts
clean:
    @echo "Cleaning..."
    rm -rf {{build_dir}}
    go clean -cache -testcache

# ── Test ──

# Run tests with race detection
test:
    @echo "Running tests..."
    go test -race -cover {{go_packages}}

# Run tests with verbose output
test-verbose:
    @echo "Running tests (verbose)..."
    go test -race -cover -v {{go_packages}}

# Run the postgres-tagged tests against a real PostgreSQL.
# Requires RELA_TEST_DATABASE_URL, e.g.:
#   RELA_TEST_DATABASE_URL=postgres://user@127.0.0.1:5432/rela_test?sslmode=disable just test-postgres
# Without it, the pgstore conformance suite skips (so this stays a no-op-safe target).
test-postgres:
    @echo "Running postgres-tagged tests (needs RELA_TEST_DATABASE_URL)..."
    go test -race -tags postgres ./internal/store/pgstore/...

# Verify the binaries compile under every backend build tag. Cheap guard
# that no build-tag seam drifted; mirrors the CI compile matrix.
build-check-tags:
    @echo "Compiling all backend build-tag combinations..."
    go build ./...
    go build -tags memorybackend ./...
    go build -tags postgres ./...
    @echo "All build-tag combinations compile."

# ── E2E Tests ──

# Install E2E test dependencies
e2e-install:
    @echo "Installing E2E test dependencies..."
    cd e2e && npm install
    cd e2e && npx playwright install chromium

# Run E2E tests (tests data entry UI via rela-server)
e2e: build-server-e2e
    @echo "Running E2E tests..."
    cd e2e && npm test

# Run E2E tests in headed mode (visible browser)
e2e-headed: build-server-e2e
    @echo "Running E2E tests (headed)..."
    cd e2e && npm run test:headed

# Run E2E tests with Playwright UI
e2e-ui: build-server-e2e
    @echo "Running E2E tests with Playwright UI..."
    cd e2e && npm run test:ui

# Run tests with coverage profile
test-coverage:
    @echo "Running tests with coverage..."
    go test -race -coverprofile=coverage.out -covermode=atomic {{go_packages}}

# Generate and display coverage report
coverage: test-coverage
    @echo "Generating coverage report..."
    go tool cover -func=coverage.out

# Check coverage meets floor thresholds (uses go-test-coverage)
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
    go test -run='^$$' -fuzz='^FuzzParseEntityID$$' -fuzztime=30s ./internal/entity/
    go test -run='^$$' -fuzz='^FuzzValidateID$$' -fuzztime=30s ./internal/entity/

# Run quick fuzz tests (5 seconds each)
fuzz-short:
    @echo "Running quick fuzz tests..."
    go test -run='^$$' -fuzz='^FuzzParseDocument$$' -fuzztime=5s ./internal/markdown/
    go test -run='^$$' -fuzz='^FuzzParseEntityID$$' -fuzztime=5s ./internal/entity/
    go test -run='^$$' -fuzz='^FuzzValidateID$$' -fuzztime=5s ./internal/entity/

# ── Lint & Format ──

# Run Go linter
lint:
    @echo "Running Go linter..."
    golangci-lint run

# Check for known vulnerabilities (govulncheck with OSV filter)
govulncheck:
    @echo "Running govulncheck..."
    scripts/govulncheck-filtered.sh

# Check architecture boundaries
arch-lint:
    @echo "Checking architecture boundaries..."
    go-arch-lint check

# Run linter with auto-fix
lint-fix:
    @echo "Running linter with auto-fix..."
    golangci-lint run --fix

# Lint markdown files
lint-md:
    @echo "Linting markdown files..."
    npx markdownlint-cli2 "**/*.md" "#node_modules" "#**/node_modules"

# Lint and fix markdown files
lint-md-fix:
    @echo "Linting and fixing markdown files..."
    npx markdownlint-cli2 --fix "**/*.md" "#node_modules" "#**/node_modules"

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

# Run all checks (lint + arch-lint + lint-md + test)
check: lint arch-lint lint-md test

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
    @echo "Installing go-arch-lint..."
    go install github.com/fe3dback/go-arch-lint@latest
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

# ── Icons ──

# Source SVG and output directories
logo_svg := "build/package/logo.svg"
icon_tmp := "build/package/.icon-tmp"

# Generate all app icons from logo.svg (requires rsvg-convert, imagemagick, iconutil)
icons: _icon-pngs _icon-icns _icon-ico _icon-linux
    @rm -rf {{icon_tmp}}
    @echo "All icons generated. Review changes with 'git diff' and commit."

# Generate intermediate PNGs at all required sizes
_icon-pngs:
    @mkdir -p {{icon_tmp}}
    @echo "Generating PNGs from {{logo_svg}}..."
    @for size in 16 32 48 64 128 256 512 1024; do \
        rsvg-convert -w $size -h $size -b '#031b75' {{logo_svg}} -o {{icon_tmp}}/icon_${size}.png; \
    done

# Generate macOS .icns (requires macOS iconutil)
_icon-icns: _icon-pngs
    @echo "Generating macOS .icns..."
    @mkdir -p {{icon_tmp}}/rela-desktop.iconset
    @cp {{icon_tmp}}/icon_16.png   {{icon_tmp}}/rela-desktop.iconset/icon_16x16.png
    @cp {{icon_tmp}}/icon_32.png   {{icon_tmp}}/rela-desktop.iconset/icon_16x16@2x.png
    @cp {{icon_tmp}}/icon_32.png   {{icon_tmp}}/rela-desktop.iconset/icon_32x32.png
    @cp {{icon_tmp}}/icon_64.png   {{icon_tmp}}/rela-desktop.iconset/icon_32x32@2x.png
    @cp {{icon_tmp}}/icon_128.png  {{icon_tmp}}/rela-desktop.iconset/icon_128x128.png
    @cp {{icon_tmp}}/icon_256.png  {{icon_tmp}}/rela-desktop.iconset/icon_128x128@2x.png
    @cp {{icon_tmp}}/icon_256.png  {{icon_tmp}}/rela-desktop.iconset/icon_256x256.png
    @cp {{icon_tmp}}/icon_512.png  {{icon_tmp}}/rela-desktop.iconset/icon_256x256@2x.png
    @cp {{icon_tmp}}/icon_512.png  {{icon_tmp}}/rela-desktop.iconset/icon_512x512.png
    @cp {{icon_tmp}}/icon_1024.png {{icon_tmp}}/rela-desktop.iconset/icon_512x512@2x.png
    @iconutil -c icns {{icon_tmp}}/rela-desktop.iconset -o build/package/macos/rela-desktop.icns

# Generate Windows .ico (requires imagemagick)
_icon-ico: _icon-pngs
    @echo "Generating Windows .ico..."
    @magick {{icon_tmp}}/icon_16.png {{icon_tmp}}/icon_32.png {{icon_tmp}}/icon_48.png \
            {{icon_tmp}}/icon_64.png {{icon_tmp}}/icon_128.png {{icon_tmp}}/icon_256.png \
            build/package/windows/rela-desktop.ico

# Generate Linux PNGs
_icon-linux: _icon-pngs
    @echo "Generating Linux PNGs..."
    @cp {{icon_tmp}}/icon_256.png build/package/linux/rela-desktop.png
    @cp {{icon_tmp}}/icon_512.png build/package/linux/rela-desktop-512.png

# ── Dev Server ──

# Run the data entry server for development (ticketing example)
[no-exit-message]
dev project="prototypes/data-entry/project" port="8080":
    go run ./cmd/rela-server -project {{project}} -port {{port}}

# Run the catalog example
[no-exit-message]
dev-catalog port="8282":
    go run ./cmd/rela-server -project prototypes/data-entry/catalog -port {{port}}

# ── Frontend Dev ──

# Run Vue dev server with hot-reloading (requires Go server running on :8080)
[no-exit-message]
dev-frontend:
    cd frontend && npm run dev

# Install frontend dependencies
install-frontend:
    cd frontend && npm install

# Build Vue frontend for production
build-frontend: install-frontend
    cd frontend && npm run build

# Build Vue frontend in development mode for E2E. This bundle has
# import.meta.env.DEV === true, so DEV-guarded test hooks (e.g. the
# backtick-autocomplete delay knob, issue #890) compile in. Production
# builds use `build-frontend`, which strips them.
build-frontend-e2e: install-frontend
    cd frontend && npm run build:e2e

# Type-check Vue frontend
typecheck-frontend:
    cd frontend && npm run typecheck

# Lint Vue frontend
lint-frontend:
    cd frontend && npm run lint
