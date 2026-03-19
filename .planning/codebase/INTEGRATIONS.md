# External Integrations

**Analysis Date:** 2026-03-19

## APIs & External Services

**GitHub:**
- Git repository cloning and operations
  - SDK/Client: `github.com/go-git/go-git/v5` (SSH, HTTPS auth)
  - Auth: GitHub OAuth device flow via `RELA_GITHUB_CLIENT_ID` environment variable
  - Supported hosts: github.com, gitlab.com, bitbucket.org
  - Usage: Desktop app can clone repos and discover rela projects within them

**Model Context Protocol (MCP):**
- Claude AI integration for editor plugins (Cursor, Claude Code, etc.)
  - SDK: `github.com/mark3labs/mcp-go` v0.43.2
  - Implementation: `internal/mcp/server.go`
  - Exposes 23 tools for entity/relation CRUD, graph analysis, schema introspection
  - Stdin/stdout protocol (runs as subprocess)
  - File watcher integration for live notifications

## Data Storage

**Databases:**
- Not used - rela is file-based

**File Storage:**
- Markdown files on local filesystem only
  - Entity files: `entities/<type>/` directory
  - Relation files: `relations/` directory
  - Config files: `metamodel.yaml`, `data-entry.yaml` in project root
  - Storage layer: `internal/storage/` package
  - Implementation: `SafeFS` wraps OS filesystem with atomic write safety
  - Watcher: `fsnotify/fsnotify` v1.9.0 for live-reload on file changes

**Caching:**
- In-memory graph cache from markdown files
  - Cache file: `.rela/cache.json` (computed, not committed)
  - Graph reconstruction: Happens on startup, synced on file watch events

**User Settings:**
- `.rela/ui-state.json` - UI state (collapsed groups, view preferences)
- `.rela/user-defaults.yaml` - User-specific form defaults (gitignored)
- Desktop preferences: `~/.config/rela/` or platform-specific preference location

## Authentication & Identity

**Auth Provider:**
- Custom: No centralized auth system
- Desktop app: Optional GitHub OAuth device flow for repository cloning
  - Flow: `internal/git/oauth.go` implements GitHub's device authorization grant
  - Scope: `repo` scope for private repository access
  - Token storage: Persisted locally (in desktop preferences, encrypted by OS)

**Authorization:**
- Not implemented - assumes single-user or trusted environment
- Web server assumes local/trusted network access

## Monitoring & Observability

**Error Tracking:**
- None - errors logged to stderr/logs

**Logs:**
- CLI: Standard error output via `log` package
- Web server: `log.Printf()` to stdout/stderr
- MCP server: Logs to stderr with `[rela-mcp]` prefix
- Desktop: Logs to platform-specific log files

## CI/CD & Deployment

**Hosting:**
- GitHub as source repository
- No cloud deployment - rela is a local tool/library

**CI Pipeline:**
- GitHub Actions (`.github/workflows/`)
  - `ci.yml`: Test, lint, coverage checks on push/PR
  - `coverage-ratchet.yml`: Coverage baseline enforcement
  - `release.yml`: Release builds and publication
  - `security.yml`: Security scanning (Dependabot)
  - `dependabot-auto-merge.yml`: Automated dependency updates

**Release Process:**
- goreleaser (`.goreleaser.yaml`) - Cross-platform binary building
- GitHub Releases for distribution
- CLI: Standalone binary
- Desktop: Platform-specific installers (macOS .dmg, Windows .exe, Linux .tar.gz)

## Environment Configuration

**Required env vars:**
- `RELA_GITHUB_CLIENT_ID` - Optional, for desktop GitHub OAuth (rela-desktop only)
- `VITE_API_BASE` - Optional, for frontend API proxy (defaults to `http://localhost:8080`)

**Optional env vars:**
- `RELA_GITHUB_CLIENT_ID` - GitHub OAuth app client ID for desktop app
- Standard Go build flags: `CGO_ENABLED`, `GOOS`, `GOARCH`

**Secrets location:**
- GitHub: Repository secrets for CI workflows
- Desktop: OS-level credential storage (Keychain on macOS, Credential Manager on Windows)
- Local: `.rela/user-defaults.yaml` (user-specific, gitignored)

## Webhooks & Callbacks

**Incoming:**
- None - rela is not a server that receives webhooks

**Outgoing:**
- None - rela does not push data to external services
- File system events trigger internal live-reload via `fsnotify`

## Version Control Integration

**Git Operations:**
- Repository cloning: `internal/git/clone.go`
  - Uses `go-git` library for HTTPS/SSH auth
  - Supports branch selection
  - Validates repository URLs before cloning
- Git status: `internal/dataentry/handlers_git.go`
  - Shows uncommitted changes in data entry app
  - Displays current branch and sync status
- Git sync/commit: Data entry app can commit changes
  - Implementation: `internal/git/git.go`
  - Used for change tracking and conflict detection

## Markdown Parsing & Rendering

**Backend:**
- `yuin/goldmark` v1.7.16 - Parse markdown entity content
- YAML frontmatter handling: Custom parsing in `internal/markdown/`
- Markdown validation: Fuzz testing in test suite

**Frontend:**
- `marked` v17.0.4 - Render markdown in browser
- `easymde` v2.20.0 - Edit markdown with rich editor
- `mermaid` v11.13.0 - Render diagrams (Mermaid syntax) in entity descriptions

## Graph Visualization

**Frontend:**
- Cytoscape 3.30.4 - Interactive graph visualization
- Used in graph view: Shows entity nodes and relation edges
- Configuration: `internal/dataentry/handlers_graph.go`

## Build & Package Management

**Dependencies:**
- `go mod` - Managed in `go.mod` with lock in `go.sum`
- npm - Managed in `frontend/package.json` and `e2e/package.json`

**Vendored Assets:**
- JS/CSS libraries vendored to `internal/dataentry/static/`
  - HTMX, EasyMDE, Slim Select, Tagify, Cytoscape, Mermaid
  - Downloaded via `just vendor-js` (curl-based)
  - Committed to git for reproducibility

## Codecov Integration

**Coverage Tracking:**
- Codecov action in CI: Uploads coverage reports
- Coverage ratchet: Enforced via `go-test-coverage` with `.coverage-baseline`
- Baseline: Committed to repo, updated on main branch merges

---

*Integration audit: 2026-03-19*
