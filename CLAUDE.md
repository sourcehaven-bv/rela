# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Test Commands

```bash
# Build
make build                    # Build binary to bin/rela
go build -o rela ./cmd/rela   # Quick build to current directory

# Test
make test                     # Run tests with race detection and coverage
go test ./...                 # Quick test
go test -v ./internal/graph/  # Single package with verbose output
go test -run TestName ./...   # Single test by name

# Lint
make lint                     # Run golangci-lint
make lint-fix                 # Auto-fix lint issues
make fmt                      # Format code (gofmt + goimports)

# Fuzz testing
make fuzz-short               # Quick fuzz tests (5s each)
make fuzz                     # Full fuzz tests (30s each)

# All checks (CI)
make ci                       # lint + test + build
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
