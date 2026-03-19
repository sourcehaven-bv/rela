# Technology Stack

**Analysis Date:** 2026-03-19

## Languages

**Primary:**
- Go 1.24 - Core CLI, server, and desktop app backend
- TypeScript 5.4 - Vue frontend development
- YAML - Entity/relation definitions and configuration

**Secondary:**
- HTML/CSS - Web UI templates
- JavaScript - Frontend scripts (embedded libraries)
- Markdown - Entity and relation content storage

## Runtime

**Environment:**
- Go 1.24+ (minimum version in go.mod)
- Node.js LTS - Frontend development and E2E testing

**Package Manager:**
- Go modules (`go mod`)
- npm (Node Package Manager) for frontend and E2E dependencies

## Frameworks

**Core Backend:**
- Standard library `net/http` - HTTP server for data entry web app
- Wails v2.11.0 - Desktop app framework (wraps Go backend with native UI)
- Model Context Protocol (MCP) via `mark3labs/mcp-go` v0.43.2 - Claude integration

**Frontend:**
- Vue 3.4.21 - Progressive web framework
- Vue Router 4.3.0 - Client-side routing
- Pinia 2.1.7 - State management

**Web UI Components:**
- HTMX 2.0.4 - HTML over HTTP for interactive pages
- EasyMDE 2.20.0 - Markdown editor widget
- Slim Select 3.4.3 - Custom dropdown selects
- Tagify 4.31.3 - Tag input component
- Cytoscape 3.30.4 - Graph visualization

**Markdown & Formatting:**
- `yuin/goldmark` v1.7.16 - Markdown parsing
- `marked` v17.0.4 - JavaScript markdown rendering (frontend)
- `mermaid` v11.13.0 - Diagram generation (Mermaid syntax)

**Testing:**
- Vitest 1.6.0 - Vue component and unit tests
- Playwright 1.41.0 - E2E tests
- Testify v1.10.0 - Go test assertions
- gopter v0.2.11 - Property-based testing for Go

**Build/Dev:**
- Vite 5.1.6 - Frontend bundler and dev server
- Just (justfile) - Task runner for project commands
- golangci-lint v1.62.2 - Go linter
- goimports - Go import formatter
- go-test-coverage - Coverage ratchet enforcement
- go-arch-lint - Architecture boundary checking

## Key Dependencies

**Critical Backend:**
- `go-git/go-git` v5.13.2 - Git operations (clone, status, commit)
- `spf13/cobra` v1.8.0 - CLI command framework
- `fsnotify/fsnotify` v1.9.0 - File system watching for live-reload
- `gopkg.in/yaml.v3` v3.0.1 - YAML parsing (entity configuration)

**Desktop:**
- `wailsapp/wails` v2.11.0 - Native desktop wrapper
- `chromedp/chromedp` v0.14.2 - Headless browser automation for data entry testing

**Utilities:**
- `fatih/color` v1.16.0 - CLI output coloring
- `olekukonko/tablewriter` v0.0.5 - Table formatting for CLI
- `samber/lo` v1.49.1 - Functional programming helpers

**Frontend Dependencies:**
- `axios` v1.6.7 - HTTP client for API calls
- `marked` v17.0.4 - Markdown rendering

## Configuration

**Environment:**
- `.env` files supported by frontend (`.env`, `.env.local`, `.env.*.local`)
- Go environment variables used for:
  - GitHub OAuth: `RELA_GITHUB_CLIENT_ID`
  - Build flags: `-ldflags` for version/build info
- Frontend environment: `VITE_API_BASE` for API base URL (defaults to `http://localhost:8080`)

**Build Configuration:**
- `justfile` - Build recipes for all three binaries (CLI, server, desktop)
- `frontend/vite.config.ts` - Vite configuration with Vue plugin and API proxy
- `cmd/rela-desktop/wails.json` - Wails configuration (not present; uses embedded options)
- `.testcoverage.yml` - Coverage enforcement thresholds per package
- `.golangci.yml` - Go linter configuration
- `.go-arch-lint.yml` - Architecture boundary rules
- `.markdownlint.yaml` - Markdown linting rules
- `.pre-commit-config.yaml` - Git pre-commit hooks

## Platform Requirements

**Development:**
- Go 1.24+ development environment
- Node.js LTS for frontend
- SQLite 3 (for some E2E scenarios, see chromedp dependency)
- System tools for icon generation (macOS: `iconutil`, Windows: ImageMagick, Linux: `rsvg-convert`)
- Git (for clone/operations)

**Desktop App:**
- macOS 10.13+: Native Cocoa framework
- Windows 10+: Windows API (requires Visual C++ runtime)
- Linux: GTK 3+ or equivalent
- CGO required for desktop build (`CGO_ENABLED=1`)
- Wails bundles platform-specific WebView engines

**Production:**
- Any OS (go binaries are cross-compiled)
- CLI runs as standalone binary
- Server runs as HTTP daemon (port configurable via `-port` flag, default 8080)
- Desktop app runs as native application

## Build Outputs

**CLI Binary:** `bin/rela` - Standalone command-line tool
**Server Binary:** `bin/rela-server` - HTTP server for data entry web app
**Desktop App:** `bin/rela-desktop` - Native desktop application (platform-specific)

All built with `-trimpath` and `-ldflags "-s -w"` for production reproducibility.

---

*Stack analysis: 2026-03-19*
