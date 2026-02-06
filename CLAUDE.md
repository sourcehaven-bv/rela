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

# Coverage Requirements (enforced in CI via go-test-coverage)
# Configured in .testcoverage.yml with ratchet baseline in .coverage-baseline
# Minimum thresholds per package — coverage can never decrease (ratchet)

# Lint
just lint                     # Run golangci-lint
just lint-fix                 # Auto-fix lint issues
just fmt                      # Format code (gofmt + goimports)

# Fuzz testing
just fuzz-short               # Quick fuzz tests (5s each)
just fuzz                     # Full fuzz tests (30s each)

# Git hooks
just install-hooks            # Install pre-commit hook (runs lint, test)

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

The project uses [go-test-coverage](https://github.com/vladopajic/go-test-coverage) with a
**coverage ratchet**: coverage can never decrease. Configuration is in `.testcoverage.yml` and
the current baseline is stored in `.coverage-baseline` (committed to the repo).

### How the Ratchet Works

- `.testcoverage.yml` defines minimum floor thresholds per package (override rules)
- `.coverage-baseline` records per-file coverage from the last merge to main/develop
- On PRs, CI checks that coverage hasn't dropped below the baseline (`diff.threshold: 0`)
- After a PR merges, CI regenerates and commits the baseline if coverage improved
- Coverage can only go up, never down

### Coverage Policy

- **Core packages** (model, errors): Floor threshold ≥95%
- **Critical functionality** (output, project, markdown, filter): Floor threshold ≥85%
- **Complex logic** (graph, metamodel, importer): Floor threshold ≥65%
- **UI/CLI code**: May have lower coverage with `coverage-ignore` comments for unreasonable-to-test code
- **Ratchet**: Actual coverage is tracked per-file in `.coverage-baseline` and must not decrease

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

**Important**: When the coverage baseline/ratchet check fails, always add
tests to improve coverage rather than updating the baseline file.

### Running Coverage Checks Locally

```bash
# Check if your changes meet coverage requirements (uses go-test-coverage)
just coverage-check

# See detailed coverage report
just coverage-html

# Install pre-commit hook (runs lint + test, coverage is checked in CI)
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
.rela/ui-state.json         # Persisted UI state (gitignored)
.rela/user-defaults.yaml    # User-specific default values (gitignored)
```

### User Defaults

The data entry app supports user-configurable default values for entity creation,
stored in `.rela/user-defaults.yaml` (gitignored, per-user). Users configure these
via the Settings page in the web UI.

**Types and resolution** (`internal/dataentry/config.go`):

- `UserDefaults` holds global property defaults, global relation defaults, and entity-type overrides
- `DefaultOverride` scopes defaults to a list of entity types
- `ResolvePropertyDefault(entityType, property)` checks overrides first, then global defaults
- `ResolveRelationDefault(entityType, relation)` same resolution order for relations

**Default resolution chain** (highest to lowest priority):

1. Entity-type override from user defaults
2. Global user default
3. Form-level default (from `data-entry.yaml`)
4. Metamodel default (from `metamodel.yaml`)

**Integration points**:

- `handleForm`: user defaults populate `ResolvedField.Default` via `coalesce()` and pre-select relations
- `handleCreate` / `handleInlineCreate`: user defaults fill empty properties and create default relations
- `handleSettings` / `handleSaveSettings`: Settings page renders metamodel-aware widgets and persists to YAML

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

## Rela for Planning & Issue Tracking

This project uses two rela instances via MCP for design and issue tracking:

- **rela-docs**: Documentation entities (concepts, features, guides, tutorials, scenarios)
- **rela-issues-and-design-tickets**: Issue tracking (tickets, features, decisions, concepts, risks, measures, tests)

### Workflow for Creating Tickets/Entities

When creating or updating entities in `rela-issues-and-design-tickets`:

1. **Create the entity** with required properties
2. **Run ALL analyze tools** to check for issues:
   - `analyze_cardinality` - check required relations
   - `analyze_orphans` - find unlinked entities
   - `analyze_properties` - validate property values
   - `analyze_validations` - run custom validation rules
3. **Fix any violations** (create missing relations, add required properties, etc.)
4. **Repeat analysis until ALL checks pass** - do not stop after fixing one issue

### Common Required Relations

| Entity Type | Required Relations |
|-------------|-------------------|
| ticket | `affects` → concept (min 1), `implements` → feature (min 1) |
| feature | `requires` → concept (min 1) |
| test-case/test-suite | `test-covers` → concept (min 1), `verifies` → feature/ticket (min 1) |
| doc-task | `affects` → concept (min 1), `triggered-by` → ticket/feature/decision (min 1), `updates` → guide/tutorial/scenario (min 1) |

### Validation Rules

The metamodel includes validation rules that enforce:

- In-progress bugs should have `why1` and `why2` started
- Done bugs must have 5-whys analysis (`why1`-`why3` required) and `prevention`
- Ready tickets need `effort`, `priority`, and `description`
- Accepted decisions need `date`, `context`, and `consequences`

Always run `analyze_validations` to catch these issues.

### 5-Whys for Bug Analysis

Bug tickets use the 5-whys method for root cause analysis:

| Property | Purpose |
|----------|---------|
| `why1` | What was the immediate cause? |
| `why2` | Why did that happen? |
| `why3` | Why did that happen? |
| `why4` | Why did that happen? |
| `why5` | What is the systemic root cause? |

Done bugs require at least 3 levels (why1-why3). The goal is to reach systemic causes
that can be addressed with process/tooling improvements documented in `prevention`.
