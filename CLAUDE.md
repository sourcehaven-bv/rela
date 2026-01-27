# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build
make build                    # Build binary to bin/rela
go build -o rela ./cmd/rela   # Quick build to current directory

# Test
make test                     # Run tests with race detection
make test-coverage            # Run tests with coverage report
make coverage                 # Generate and display coverage report
make coverage-check           # Check coverage meets minimum thresholds
make coverage-html            # Generate HTML coverage report
go test ./...                 # Quick test
go test -v ./internal/graph/  # Single package with verbose output
go test -run TestName ./...   # Single test by name

# Coverage Requirements (enforced in CI)
# - Total project coverage: ≥45.0%
# - internal/model: ≥95.0%
# - internal/errors: ≥95.0%
# - internal/output: ≥90.0%
# - internal/project: ≥85.0%
# - internal/markdown: ≥85.0%
# - internal/filter: ≥85.0%
# - internal/graph: ≥75.0%
# - internal/metamodel: ≥65.0%
# - internal/importer: ≥65.0%

# Lint
make lint                     # Run golangci-lint
make lint-fix                 # Auto-fix lint issues
make fmt                      # Format code (gofmt + goimports)

# Fuzz testing
make fuzz-short               # Quick fuzz tests (5s each)
make fuzz                     # Full fuzz tests (30s each)

# Git hooks
make install-hooks            # Install pre-commit hook (runs lint, test, coverage)

# All checks (CI)
make ci                       # lint + test + coverage-check + build
```

## Architecture Overview

rela is a traceability CLI that manages entities and their relationships. Common use cases include requirements, decisions, solutions, and components. Data is stored as markdown files with YAML frontmatter.

### Core Data Flow

```
metamodel.yaml → Metamodel (defines entity types, relations, properties)
                     ↓
entities/*.md  → Entity → Graph (in-memory)
relations/*.md → Relation ↗      ↓
                              Cache (.rela/cache.json)
```

### Package Structure

| Package | Purpose |
|---------|---------|
| `cmd/rela` | Entry point |
| `internal/cli` | Cobra commands (create, link, analyze, export, etc.) |
| `internal/tui` | Bubbletea TUI screens (browser, detail, search, etc.) |
| `internal/model` | Core types: `Entity`, `Relation`, `Status` |
| `internal/graph` | In-memory graph with adjacency lists, tracing, analysis |
| `internal/metamodel` | Schema loading, validation, custom types |
| `internal/markdown` | Parse/write entities and relations from markdown files |
| `internal/project` | Project discovery, paths (`Context`) |
| `internal/output` | CLI output formatting (table, JSON) |
| `internal/filter` | Entity filtering by properties |

### Key Types

- **Entity** (`internal/model/entity.go`): Architecture artifact with ID, type, properties map
- **Relation** (`internal/model/entity.go`): Directed edge between two entities
- **Graph** (`internal/graph/graph.go`): Thread-safe graph with nodes/edges and adjacency maps
- **Metamodel** (`internal/metamodel/types.go`): Schema defining entity types, relations, and property types
- **Context** (`internal/project/context.go`): Project paths (root, entities/, relations/, templates/, .rela/)

### TUI Architecture

The TUI uses Bubbletea with a central `App` struct containing screen-specific models:
- `Screen` enum controls which view is active
- Each screen (browser, detail, search, etc.) has its own model file
- Navigation uses a screen stack for back navigation

### CLI State Initialization

`internal/cli/root.go` sets up shared state in `PersistentPreRunE`:
1. Discover project root (find `metamodel.yaml`)
2. Load metamodel
3. Initialize graph (from cache or by syncing markdown files)

All commands share `projectCtx`, `meta`, `g` (graph), and `out` (output writer).

## Test Coverage

The project maintains high test coverage for core business logic. Coverage thresholds are enforced in CI.

### Coverage Policy

- **Core packages** (model, errors): Must maintain ≥95% coverage
- **Critical functionality** (output, project, markdown, filter): Must maintain ≥85% coverage
- **Complex logic** (graph, metamodel): Must maintain ≥65% coverage
- **UI/CLI code**: May have lower coverage with `coverage-ignore` comments for unreasonable-to-test code

### Coverage-Ignore Comments

Use `coverage-ignore` comments for code that is unreasonable to unit test:

```go
// coverage-ignore: main function - entry point, tested via integration tests
func main() {
    // ...
}

// coverage-ignore: TUI code - requires interactive terminal
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    // ...
}

// coverage-ignore: requires external graphviz installation
func renderWithGraphviz() error {
    // ...
}
```

Valid reasons for `coverage-ignore`:
- Main/entry point functions (better tested via integration tests)
- Interactive TUI code (requires terminal simulation)
- External tool dependencies (graphviz, etc.)
- OS-specific functionality that can't be reliably mocked

### Running Coverage Checks Locally

```bash
# Check if your changes meet coverage requirements
make coverage-check

# See detailed coverage report
make coverage-html

# Install pre-commit hook to check before commit
make install-hooks
```

## Lint Configuration

The project uses golangci-lint with extensive rules. Key exclusions:
- TUI code is exempt from complexity checks (funlen, gocognit, nestif)
- Test files exempt from dupl, funlen, magic numbers
- Cobra `cmd`/`args` unused parameters are allowed
- Line length limit: 120 chars

## Project Files

```
metamodel.yaml              # Entity/relation schema
entities/<type>/            # Markdown entity files by type
relations/                  # Markdown relation files (FROM--type--TO.md)
templates/entities/<type>.md  # Optional: entity templates for defaults
templates/relations/<type>.md # Optional: relation templates for defaults
.rela/cache.json            # Graph cache (gitignored)
```
