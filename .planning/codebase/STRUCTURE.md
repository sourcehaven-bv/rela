# Codebase Structure

**Analysis Date:** 2026-03-19

## Directory Layout

```
project-root/
├── cmd/                          # Binary entry points
│   ├── rela/                      # CLI binary (main.go)
│   ├── rela-server/               # HTTP server binary (main.go)
│   └── rela-desktop/              # Wails desktop app (main.go, welcome.go)
├── internal/                      # Private packages (no external imports)
│   ├── attachment/                # Attachment handling
│   ├── automation/                # Automation engine
│   ├── cli/                       # Cobra CLI commands (create, link, analyze, export, etc.)
│   ├── conflict/                  # Git conflict detection and resolution
│   ├── dataentry/                 # Web app handlers, templates, static files
│   │   ├── api_v1.go              # REST API v1 endpoints
│   │   ├── handlers_*.go          # Route handlers (list, entity, form, create, etc.)
│   │   ├── views_*.go             # Template rendering logic
│   │   ├── router.go              # HTTP route registration
│   │   ├── app.go                 # App struct, initialization, reload coordination
│   │   ├── config.go              # Data entry config (data-entry.yaml)
│   │   ├── events.go              # SSE event broker
│   │   ├── static/                # Embedded static files (templates, Vue v2 build)
│   │   └── templates/             # HTML templates for server-rendered views
│   ├── dataentryconfig/           # Data entry config parsing and validation
│   ├── desktop/                   # Wails desktop app integration
│   ├── errors/                    # Domain error types and sentinels
│   ├── filter/                    # Entity filtering by properties
│   ├── git/                       # Git operations (status, commit, sync)
│   ├── graph/                     # In-memory entity/relation graph
│   ├── htmlutil/                  # HTML utilities (escaping, rendering)
│   ├── importer/                  # Import from external formats
│   ├── markdown/                  # YAML frontmatter parsing and entity/relation serialization
│   ├── mcp/                       # Model Context Protocol server
│   ├── metamodel/                 # Schema loader, validation, property type system
│   ├── migration/                 # YAML-level schema migrations for metamodel.yaml and config
│   ├── model/                     # Core types (Entity, Relation, Status, SyncResult)
│   ├── natsort/                   # Natural sort order (1, 2, 10 not 1, 10, 2)
│   ├── openapi/                   # OpenAPI spec generation from metamodel
│   ├── output/                    # CLI output formatting (table, JSON)
│   ├── project/                   # Project discovery and path context
│   ├── rename/                    # Entity/relation renaming operations
│   ├── repository/                # Domain-level persistence interface and implementation
│   ├── search/                    # Full-text and structured search
│   ├── storage/                   # Filesystem abstraction (FS interface, OsFS, SafeFS)
│   ├── testutil/                  # Test fixtures and helpers
│   ├── views/                     # View configuration (views.yaml)
│   └── workspace/                 # Stateful domain session (graph, repo, metamodel coordination)
├── frontend/                      # Vue 3 SPA (data entry web app)
│   ├── src/
│   │   ├── components/            # Vue components (entity detail, forms, lists, etc.)
│   │   ├── composables/           # Vue composables (useEvents, useKeyboardShortcuts, etc.)
│   │   ├── router/                # Vue Router (views)
│   │   ├── stores/                # Pinia stores (schema, entities, ui, git)
│   │   ├── types/                 # TypeScript types (Entity, Schema, Config)
│   │   ├── utils/                 # Helper functions
│   │   ├── api/                   # API client functions
│   │   ├── test/                  # Test setup and fixtures
│   │   └── main.ts                # Vue app bootstrap
│   ├── e2e/                       # Playwright end-to-end tests
│   └── package.json               # Node.js dependencies (Vue, TypeScript, etc.)
├── e2e/                           # End-to-end tests (Playwright, Go helpers)
├── prototypes/                    # Example projects for testing (ticketing, catalog)
├── templates/                     # Project templates (entity/relation defaults)
├── entities/                      # Main rela project's entities (documentation)
├── tickets/                       # Issue tracking rela project (design tickets)
├── docs/                          # Documentation, design docs, tutorials
├── docs-project/                  # Documentation rela project (guides, tutorials, concepts)
├── examples/                      # Example config files (views.yaml)
├── build/                         # Release packaging (homebrew, Linux, macOS, Windows)
├── .github/                       # GitHub Actions CI/CD workflows
├── .planning/                     # Planning documents and analysis (this file)
├── metamodel.yaml                 # Main project metamodel (entity and relation schema)
├── go.mod, go.sum                 # Go module dependencies
├── go.mod                         # Go version and dependencies
├── justfile                       # Build/test/lint commands (just build, just test, etc.)
├── .golangci.yml                  # golangci-lint configuration
├── .testcoverage.yml              # Coverage thresholds (ratchet baseline)
├── .coverage-baseline             # Coverage baseline per-file
└── README.md                      # Project overview
```

## Directory Purposes

**cmd/ - Binary Entry Points**
- Purpose: Three entry points for different deployment modes
- Contains: main.go files for CLI, HTTP server, and Wails desktop app
- Key files:
  - `cmd/rela/main.go`: CLI binary, calls `cli.Execute()`
  - `cmd/rela-server/main.go`: HTTP server, creates App and starts listener
  - `cmd/rela-desktop/main.go`: Wails app integration

**internal/cli - CLI Commands**
- Purpose: Cobra-based CLI command implementations
- Contains: Command definitions with PersistentPreRunE for state initialization
- Key files:
  - `root.go`: Root command with shared state (workspace, metamodel, graph)
  - `create.go`: Create entity command
  - `link.go`: Create relation command
  - `analyze.go`: Graph analysis command
  - `export.go`: Export command
  - Plus 10+ other commands (list, delete, describe, validate, etc.)

**internal/dataentry - Web App**
- Purpose: HTTP handlers, templates, and static assets for data entry web UI
- Contains: Request handlers, template rendering, configuration, event streaming
- Key files:
  - `app.go`: App struct, initialization, live-reload coordination
  - `router.go`: Route registration (NewRouter method returns http.Handler)
  - `handlers.go`: Index, list, entity, form handlers
  - `handlers_git.go`: Git status, sync handlers
  - `handlers_conflict.go`: Conflict detection and resolution
  - `api_v1.go`: JSON API endpoints (legacy and v1)
  - `views_*.go`: Template rendering for different page types
  - `config.go`: Data entry config loading and validation
  - `static/`: Embedded static files (HTML templates, Vue v2 build artifacts)
  - `templates/`: HTML templates using Go text/template

**internal/workspace - Domain Session**
- Purpose: Stateful session coordinating repository, graph, and metamodel
- Contains: Workspace struct, read/write methods, transaction coordination, watcher
- Key files:
  - `workspace.go`: Workspace struct, New(), write operations
  - `document.go`: Read operations (Entity, Relation, Entities, Relations, etc.)
  - `query.go`: Graph query methods (e.g., OutgoingRelations)
  - `migrate.go`: Schema migration helpers
  - `init.go`: Project initialization
  - `validate.go`: Validation helpers

**internal/repository - Persistence Layer**
- Purpose: Domain-level CRUD for entities, relations, cache, metamodel
- Contains: Repository struct implementing Store interface, transaction coordinator
- Key files:
  - `repository.go`: Repository struct, Store interface definition, CRUD methods
  - `transaction.go`: Transaction interface, transaction struct with two-phase commit
  - `change.go`: Change/event types for file watcher callbacks
  - `repository_test.go`: Tests

**internal/graph - In-Memory Query Engine**
- Purpose: Thread-safe in-memory graph with adjacency lists and property indexing
- Contains: Graph struct with nodes, edges, adjacency maps, query methods
- Key files:
  - `graph.go`: Graph struct, AddNode, AddEdge, query methods
  - `trace.go`: Graph traversal (Trace, TraceBidirectional, TraceAcyclic)
  - `filter.go`: Filtering and analysis operations
  - Additional files for specific graph operations

**internal/metamodel - Schema Definition**
- Purpose: Load and validate schema for entity/relation types and properties
- Contains: Metamodel struct, EntityDef, RelationDef, PropertyDef
- Key files:
  - `loader.go`: Load metamodel from YAML
  - `metamodel.go`: Metamodel struct, alias resolution, validation
  - `types.go`: EntityDef, RelationDef, PropertyDef type definitions
  - `errors.go`: Metamodel-specific error types

**internal/markdown - File Format**
- Purpose: Parse and serialize markdown documents with YAML frontmatter
- Contains: Document parsing, entity/relation marshaling, content handling
- Key files:
  - `parser.go`: ParseDocument, splitFrontmatter, conflict detection
  - `entity.go`: Entity frontmatter to/from markdown
  - `relation.go`: Relation frontmatter to/from markdown
  - `fileio.go`: ReadEntity, WriteEntity, ReadRelation, WriteRelation
  - `content.go`: Markdown content parsing and formatting
  - `sync.go`: Sync entities/relations from directory trees
  - `normalize.go`: Path normalization

**internal/project - Project Context**
- Purpose: Locate project root and compute standardized paths
- Contains: Context struct with path constants
- Key files:
  - `context.go`: Context struct, Discover function, path helpers

**internal/storage - Filesystem Abstraction**
- Purpose: Abstracted filesystem interface for testability
- Contains: FS interface, OsFS implementation, SafeFS wrapper
- Key files:
  - `fs.go`: FS interface definition and OsFS implementation
  - `safefs.go`: SafeFS wrapper for atomic writes (rename-based commit)

**internal/model - Core Types**
- Purpose: Domain model definitions
- Contains: Entity, Relation, Status, SyncResult types
- Key files:
  - `entity.go`: Entity struct, helpers (GetString, SetString, Title, Description)
  - `relation.go`: Relation struct (From, Type, To)
  - `status.go`: Status constants (Open, InProgress, Done, etc.)
  - `sync.go`: SyncResult struct

**internal/errors - Error Types**
- Purpose: Domain-level error definitions
- Contains: Sentinel errors and custom error types
- Key files:
  - `errors.go`: ErrNotFound, ErrInvalidType, EntityNotFoundError, etc.

**internal/filter - Entity Filtering**
- Purpose: Filter entities by property values
- Contains: Filter config and execution
- Key files:
  - `filter.go`: ApplyFilters, filter operators (=, !=, contains, etc.)

**internal/git - Git Operations**
- Purpose: Git status, commit, sync operations
- Contains: Git wrapper around go-git library
- Key files:
  - `git.go`: Ops struct, Status, Commit, Sync methods

**internal/mcp - MCP Server**
- Purpose: Model Context Protocol server for AI assistant integration
- Contains: Server struct, tool/resource/prompt registrations
- Key files:
  - `server.go`: Server struct, NewServer, Serve
  - `tools.go`: Tool registrations
  - `resources.go`: Resource registrations
  - `prompts.go`: Prompt registrations

**frontend/ - Vue 3 SPA**
- Purpose: Single Page Application for data entry web UI (desktop and browser)
- Contains: Vue components, composables, stores, routing, types
- Structure:
  - `src/components/`: Vue components (EntityDetail, EntityList, Forms, etc.)
  - `src/composables/`: Reusable logic (useEvents, useKeyboardShortcuts)
  - `src/stores/`: Pinia state management (schema, entities, ui state)
  - `src/router/`: Vue Router configuration
  - `src/types/`: TypeScript type definitions
  - `src/api/`: API client functions
  - `src/main.ts`: Bootstrap script
  - `e2e/`: Playwright tests

## Key File Locations

**Entry Points:**
- `cmd/rela/main.go`: CLI entry point
- `cmd/rela-server/main.go`: HTTP server entry point
- `cmd/rela-desktop/main.go`: Desktop app entry point
- `internal/cli/root.go`: CLI command dispatcher and global state initialization
- `internal/dataentry/router.go`: Web app route registration

**Configuration:**
- `metamodel.yaml`: Project schema (entity types, relation types, properties)
- `data-entry.yaml`: Web app configuration (navigation, lists, forms, views)
- `views.yaml`: View definitions for custom entity display
- `.rela/user-defaults.yaml`: User-specific entity/relation default values
- `templates/entities/<type>.md`: Entity template defaults
- `templates/relations/<type>.md`: Relation template defaults

**Core Logic:**
- `internal/graph/graph.go`: In-memory query engine
- `internal/workspace/workspace.go`: Write-through domain session
- `internal/repository/repository.go`: Persistence abstraction
- `internal/markdown/parser.go`: Markdown parsing
- `internal/metamodel/loader.go`: Schema loading

**Testing:**
- `internal/testutil/`: Test helpers and fixtures
- `frontend/src/test/`: Frontend test setup
- `e2e/`: End-to-end tests
- Files named `*_test.go`: Unit tests (Go)
- Files named `*.test.ts`: Unit tests (TypeScript)

## Naming Conventions

**Files:**
- `handlers_<domain>.go`: HTTP handlers for a specific domain (git, conflict, etc.)
- `views_<type>.go`: Template rendering for a specific view type
- `api_<version>.go`: API implementations for different versions
- `*_test.go`: Go unit tests
- `*.test.ts`: TypeScript unit tests
- Entity files: `entities/<type>/<id>.md` (e.g., `entities/feature/user-auth.md`)
- Relation files: `relations/<from>--<type>--<to>.md` (e.g., `relations/user-auth--depends-on--data-model.md`)

**Directories:**
- Package names match domain concept (graph, workspace, metamodel, repository)
- Plural for collections: `components/`, `stores/`, `handlers/` (implied in filename patterns)
- `internal/` prefix for private packages
- `cmd/` for binary entry points
- Underscore for separation in multi-word packages: `dataentry`, `dataentryconfig`

**Functions and Types:**
- Entity types: PascalCase in code, kebab-case in markdown filenames (`requirement`, `design-decision`)
- Handler methods: `handleXxx()` (handleList, handleEntity, handleCreate)
- Config types: `XxxConfig` (DataEntryConfig, FilterConfig, ListConfig)
- Error types: `XxxError` (EntityNotFoundError, ValidationError)
- Package functions: lowercase start, no receiver prefix (NewEntity, ParseDocument)

## Where to Add New Code

**New Feature (Entity Type, Relation Type):**
- Define in: `metamodel.yaml` (add EntityDef or RelationDef)
- Entity template: `templates/entities/<type>.md` (optional default values)
- Relation template: `templates/relations/<type>.md` (optional default values)
- Data entry form: `data-entry.yaml` (add List, Form, or View if needed)
- Entity directory: `entities/<type>/` (created automatically by CLI)
- Tests: Add test fixtures in existing `*_test.go` files or new test files in `internal/` package

**New HTTP Handler:**
- Implementation: `internal/dataentry/handlers_<domain>.go` (group related handlers)
- Route registration: `internal/dataentry/router.go` (add to NewRouter method)
- Template: `internal/dataentry/templates/<name>.html` (if server-rendered)
- Static file: `internal/dataentry/static/<name>` (if client-side)

**New CLI Command:**
- Implementation: `internal/cli/<command>.go` (one file per command or group related commands)
- Command registration: `internal/cli/root.go` (add to init() or other init functions)
- Tests: `internal/cli/<command>_test.go`

**New Graph Query:**
- Implementation: Add method to Graph struct in `internal/graph/graph.go` or separate file
- Tests: `internal/graph/<feature>_test.go`

**New Validator or Filter:**
- Implementation: `internal/filter/filter.go` (add operator) or `internal/workspace/validate.go` (add validation)
- Tests: Existing `*_test.go` files

**Utilities and Helpers:**
- Shared helpers: `internal/htmlutil/`, `internal/natsort/`, `internal/output/`
- Domain-specific: Place with the package that uses them
- Test helpers: `internal/testutil/`

## Special Directories

**entities/ and relations/**
- Purpose: Project data (actual entities and relations managed by rela)
- Generated: Yes (created by `rela create`, `rela link` commands)
- Committed: Yes (data files committed to git)
- Organization: `entities/<type>/<id>.md` and `relations/<from>--<type>--<to>.md`

**.rela/**
- Purpose: Cache and state files
- Generated: Yes (created automatically)
- Committed: No (gitignored)
- Contains:
  - `cache.json`: In-memory graph serialized for fast startup
  - `ui-state.json`: Web app state (expanded/collapsed sections)
  - `user-defaults.yaml`: User-specific default values
  - `workspace-state.json`: Workspace internal state (optional)

**templates/entities/ and templates/relations/**
- Purpose: Default values and templates for new entities/relations
- Generated: No (created manually by users)
- Committed: Yes
- Usage: CLI uses template files as defaults when creating entities

**internal/dataentry/static/**
- Purpose: Embedded static files (CSS, JavaScript, images, HTML)
- Generated: Yes (built from frontend/ via build process)
- Committed: No (generated, gitignored)
- Contains:
  - `v2/`: Vue v2 SPA build output
  - `index.html`: Server-rendered HTML template index
  - Images, CSS (minimal, most styling in Vue v2)

**prototypes/data-entry/**
- Purpose: Example projects for testing and demonstration
- Generated: No (created manually for testing different configs)
- Committed: Yes
- Usage: Tests and manual verification of different data-entry.yaml configurations

**docs/, docs-project/**
- Purpose: Project documentation
- Generated: No (created manually)
- Committed: Yes
- docs/: Design documents, tutorials, scenarios
- docs-project/: Rela project for documentation entities (concepts, guides, etc.)

---

*Structure analysis: 2026-03-19*
