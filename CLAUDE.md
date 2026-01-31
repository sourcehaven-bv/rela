# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build
just build                    # Build all binaries (CLI + server + desktop)
just build-cli                # Build CLI binary to bin/rela
just build-server             # Build data entry server to bin/rela-server
just build-desktop            # Build Wails desktop app to bin/rela-desktop
go build -o rela ./cmd/rela   # Quick build to current directory

# Test
just test                     # Run tests with race detection
just test-coverage            # Run tests with coverage report
just coverage                 # Generate and display coverage report
just coverage-check           # Check coverage meets minimum thresholds
just coverage-html            # Generate HTML coverage report
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
# - internal/dataentry: ≥60.0%
# - internal/tui: ≥15.0% (interactive TUI code)
# - All other tested packages: ≥30.0% (general minimum)

# Lint
just lint                     # Run golangci-lint
just lint-fix                 # Auto-fix lint issues
just fmt                      # Format code (gofmt + goimports)

# Fuzz testing
just fuzz-short               # Quick fuzz tests (5s each)
just fuzz                     # Full fuzz tests (30s each)

# Git hooks
just install-hooks            # Install pre-commit hook (runs lint, test, coverage)

# All checks (CI)
just ci                       # lint + test + coverage-check + build

# Dev server
just dev                      # Run data entry server (ticketing example on :8080)
just dev-catalog              # Run catalog example on :8282
```

## Architecture Overview

rela is a traceability CLI that manages entities and their relationships. Common use cases include
requirements, decisions, solutions, and components. Data is stored as markdown files with YAML frontmatter.

### Core Data Flow

```text
metamodel.yaml → Metamodel (defines entity types, relations, properties)
                     ↓
entities/*.md  → Entity → Graph (in-memory)
relations/*.md → Relation ↗      ↓
                              Cache (.rela/cache.json)
```

### Package Structure

| Package              | Purpose                                                 |
| -------------------- | ------------------------------------------------------- |
| `cmd/rela`           | CLI entry point                                         |
| `cmd/rela-server`    | Data entry HTTP server entry point                      |
| `cmd/rela-desktop`   | Wails desktop app entry point                           |
| `internal/cli`       | Cobra commands (create, link, analyze, export, etc.)    |
| `internal/tui`       | Bubbletea TUI screens (browser, detail, search, etc.)   |
| `internal/dataentry` | Config-driven data entry web app (HTMX, handlers, views)|
| `internal/model`     | Core types: `Entity`, `Relation`, `Status`              |
| `internal/graph`     | In-memory graph with adjacency lists, tracing, analysis |
| `internal/metamodel` | Schema loading, validation, custom types                |
| `internal/markdown`  | Parse/write entities and relations from markdown files  |
| `internal/project`   | Project discovery, paths (`Context`)                    |
| `internal/output`    | CLI output formatting (table, JSON)                     |
| `internal/filter`    | Entity filtering by properties                          |
| `internal/mcp`       | MCP server: tools, resources, prompts, file watcher     |
| `internal/migration` | Schema migration system for project files               |

### Key Types

- **Entity** (`internal/model/entity.go`): Architecture artifact with ID, type, properties map
- **Relation** (`internal/model/entity.go`): Directed edge between two entities
- **Graph** (`internal/graph/graph.go`): Thread-safe graph with nodes/edges and adjacency maps
- **Metamodel** (`internal/metamodel/types.go`): Schema defining entity types, relations, and property types
- **Context** (`internal/project/context.go`): Project paths (root, entities/, relations/, templates/, .rela/)
- **Server** (`internal/mcp/server.go`): MCP server wrapping graph, metamodel, and project context

### MCP Server

The `rela mcp` command starts a Model Context Protocol server over stdio, exposing rela's
capabilities to AI assistants (Claude Code, Cursor, etc.):

- **22 tools**: Entity/relation CRUD, graph tracing, analysis, schema introspection, export
- **3 resources**: `rela://metamodel`, `rela://entity/{type}/{id}`, `rela://relation/{from}/{type}/{to}`
- **4 prompts**: `analyze-traceability`, `review-orphans`, `summarize-project`, `review-entity`
- **File watcher**: Watches entities/, relations/, and metamodel.yaml with 200ms debounce

The `rela mcp` command handles its own initialization (project discovery, metamodel loading, graph
sync) independently from the standard CLI state setup in `PersistentPreRunE`.

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
just coverage-check

# See detailed coverage report
just coverage-html

# Install pre-commit hook to check before commit
just install-hooks
```

## Lint Configuration

The project uses golangci-lint with extensive rules. Key exclusions:

- TUI code is exempt from complexity checks (funlen, gocognit, nestif)
- Test files exempt from dupl, funlen, magic numbers
- Cobra `cmd`/`args` unused parameters are allowed
- Line length limit: 120 chars

## Project Files

```text
metamodel.yaml              # Entity/relation schema
entities/<type>/            # Markdown entity files by type
relations/                  # Markdown relation files (FROM--type--TO.md)
templates/entities/<type>.md  # Optional: entity templates for defaults
templates/relations/<type>.md # Optional: relation templates for defaults
.rela/cache.json            # Graph cache (gitignored)
```

## Migration System

The migration system (`internal/migration/`) handles schema evolution for project files like
`metamodel.yaml`. It uses AST-level YAML transformations to preserve comments and formatting.

### Architecture

```text
Migration Interface
       ↓
   Registry (ordered list of migrations)
       ↓
   Runner (detect → apply → write)
       ↓
   yaml.Node helpers (AST manipulation)
```

### Key Types

- **Migration** (`migration.go`): Interface for schema migrations
- **FileType** (`migration.go`): Enum for file types (`metamodel`, `views`, etc.)
- **Result** (`runner.go`): Result of applying a single migration
- **FileResult** (`runner.go`): Result of migrating a file

### Adding a New Migration

1. Create a new file in `internal/migration/` (e.g., `my_migration.go`)

2. Implement the `Migration` interface:

```go
package migration

import "gopkg.in/yaml.v3"

func init() {
    Register(&MyMigration{})
}

type MyMigration struct{}

func (m *MyMigration) Name() string {
    return "my-migration"
}

func (m *MyMigration) Description() string {
    return "Description shown to users"
}

func (m *MyMigration) FileTypes() []FileType {
    return []FileType{FileTypeMetamodel}
}

func (m *MyMigration) Detect(doc *yaml.Node) bool {
    // Return true if this migration should be applied
    root := GetDocumentRoot(doc)
    // Use yaml_util.go helpers to inspect the document
    return false
}

func (m *MyMigration) Apply(doc *yaml.Node) error {
    // Transform the document in-place
    root := GetDocumentRoot(doc)
    // Use yaml_util.go helpers to modify the document
    return nil
}
```

1. Add tests in `internal/migration/my_migration_test.go`

### yaml.Node Helpers

`yaml_util.go` provides helpers for safe AST manipulation:

| Function                                    | Purpose                             |
| ------------------------------------------- | ----------------------------------- |
| `GetDocumentRoot(doc)`                      | Get root mapping from document node |
| `GetMapValue(node, key)`                    | Get value node by key               |
| `SetMapValue(node, key, val)`               | Set/add value in mapping            |
| `RenameMapKey(node, old, new)`              | Rename a key                        |
| `FindMapEntriesByKey(node, key)`            | Find all entries with key           |
| `ReplaceMapValueByKey(node, key, old, new)` | Replace values by key               |
| `WalkMappings(node, fn)`                    | Walk all mapping nodes              |

### Integration with Loader

The metamodel loader (`internal/metamodel/loader.go`) checks for migrations on load:

1. Before parsing, it runs `migration.Detect()` on the file
2. If migrations are detected, it returns a `migration.Error`
3. The error message tells users to run `rela migrate`

Commands that don't need the metamodel (init, migrate, mcp, version, etc.) are excluded from this check in `internal/cli/root.go`.

## Working Documents

Place temporary working documents in the `.ignored/` directory:

- Design documents
- Bug/ticket tracking files
- QA reports
- Test reports
- Scratch notes

This directory is gitignored. Never commit design docs, tickets, or reports to the repository.
