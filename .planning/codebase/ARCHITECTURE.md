# Architecture

**Analysis Date:** 2026-03-19

## Pattern Overview

**Overall:** Layered domain-driven architecture with clear separation between persistence, domain logic, and presentation

**Key Characteristics:**
- **Write-through workspace pattern**: All write operations go through workspace to keep disk and memory in sync
- **In-memory graph with markdown persistence**: Entities stored as markdown files with YAML frontmatter, loaded into in-memory graph
- **Abstraction boundaries via repository pattern**: Storage abstraction allows different backends (filesystem, database, API)
- **Transaction semantics**: Two-phase commit for atomic multi-file writes
- **Plugin architecture for CLI**: Cobra-based command framework with shared state initialization

## Layers

**Presentation & Handlers:**
- Purpose: HTTP handlers, CLI commands, UI rendering (templates), event streaming
- Location: `internal/dataentry` (web handlers), `internal/cli` (CLI commands), `frontend/src` (Vue SPA)
- Contains: HTTP route handlers, template rendering, form processing, SSE event broker, CLI command implementations
- Depends on: Workspace, metamodel, graph, markdown
- Used by: External clients (browsers, CLI users)

**Domain Session (Workspace):**
- Purpose: Stateful session that owns repository, graph, metamodel, and automation engine; enforces write-through semantics
- Location: `internal/workspace/workspace.go`
- Contains: Workspace struct, write operations (CreateEntity, UpdateEntity, DeleteEntity), transaction coordination, watcher integration
- Depends on: Repository, graph, metamodel, markdown
- Used by: CLI commands, dataentry handlers, MCP server

**Persistence Layer (Repository):**
- Purpose: Domain-level CRUD for entities, relations, cache, metamodel, templates; abstracts storage mechanism
- Location: `internal/repository/repository.go`
- Contains: Entity/relation/cache CRUD, sync operations, transaction interface, path resolution
- Depends on: Storage FS interface, markdown IO, metamodel
- Used by: Workspace, dataentry app initialization

**In-Memory Query Engine (Graph):**
- Purpose: Thread-safe in-memory graph with adjacency lists, property indexing, and traversal operations
- Location: `internal/graph/graph.go`
- Contains: Node/edge storage, adjacency maps, property index, query methods (OutgoingRelations, IncomingRelations, NodesByType)
- Depends on: Model types
- Used by: Workspace queries, handlers for filtering and analysis

**Schema & Validation (Metamodel):**
- Purpose: Load and validate schema for entity types, relation types, and custom properties; resolve aliases
- Location: `internal/metamodel/loader.go`, `internal/metamodel/metamodel.go`
- Contains: EntityDef, RelationDef, PropertyDef, validation rules, alias resolution
- Depends on: YAML parsing, model types
- Used by: Repository (path resolution), workspace, handlers (entity creation)

**File Format & Serialization (Markdown):**
- Purpose: Parse/write markdown documents with YAML frontmatter; handle git conflicts and migrations
- Location: `internal/markdown/parser.go`, `internal/markdown/entity.go`, `internal/markdown/relation.go`, `internal/markdown/fileio.go`
- Contains: Document parsing, entity/relation marshaling, content handling, sync operations
- Depends on: Model types, YAML parsing
- Used by: Repository for entity/relation I/O

**Storage Abstraction:**
- Purpose: Provide filesystem interface that can be swapped (OsFS for production, MemFS for testing)
- Location: `internal/storage/fs.go`, `internal/storage/safefs.go`
- Contains: FS interface, OsFS implementation, SafeFS wrapper (atomic writes)
- Depends on: os package
- Used by: Repository, project discovery

**Project Context & Discovery:**
- Purpose: Locate project root (find metamodel.yaml), compute paths for entities/relations/templates/cache
- Location: `internal/project/context.go`
- Contains: Context struct with path constants, Discover function (walk up tree for metamodel.yaml)
- Depends on: Storage FS interface
- Used by: Repository, workspace initialization, CLI startup

**Data Models:**
- Purpose: Core types for entities, relations, properties, sync results
- Location: `internal/model/entity.go`, `internal/model/relation.go`, `internal/model/status.go`
- Contains: Entity struct (ID, Type, Properties, Content), Relation struct (From, Type, To), helpers
- Depends on: Standard library only
- Used by: All layers above storage

## Data Flow

**Read Entity Flow:**

1. CLI/handler calls `workspace.Entity(entityType, id)`
2. Workspace queries `graph.Node(id)` (in-memory hit)
3. Graph returns cached entity
4. Handler renders/outputs entity

**Create Entity Flow:**

1. Handler receives form data
2. Handler calls `workspace.CreateEntity(entity, relations)`
3. Workspace starts transaction: `repo.Transaction(fn)`
   - Transaction stages entity write to temporary file (.new)
   - Stages relation writes to temporary files
   - On commit: all files renamed atomically
4. On success: workspace reloads graph from new files
5. Handler returns success response

**Sync (Load) Flow:**

1. CLI starts: `workspace.New(repo)` or workspace initialization
2. Workspace checks cache: `repo.CacheExists()`
3. If cache exists: `repo.LoadCache(graph)` (fast path, loads .rela/cache.json)
4. If cache missing or error: `repo.Sync(meta, graph)` (full path)
   - Sync walks entities/ and relations/ directories
   - Parses markdown files into Entity/Relation structs
   - Adds to graph via AddNode/AddEdge
   - Returns SyncResult (new/modified/deleted counts)
5. Graph now contains all entities and relations in memory

**Live-Reload Flow (Data Entry Web App):**

1. File watcher detects change in entities/, relations/, or metamodel.yaml
2. Watcher calls registered callback (OnReload)
3. Workspace reloads affected files via `repo.Sync(meta, graph)`
4. App rebuilds templates and style maps
5. Event broker sends SSE notification to all connected browsers
6. Browsers reload with new state

**State Management:**

- **Disk state**: Markdown files in entities/, relations/, metamodel.yaml
- **Memory state**: In-memory graph in workspace, reloaded on startup and on file changes
- **Cache state**: .rela/cache.json (optional, for fast startup; regenerated on write)
- **UI state**: Browser localStorage (managed by Vue) and .rela/ui-state.json
- **User defaults**: .rela/user-defaults.yaml (per-user, gitignored)

Write operations always update disk first (to file), then reload graph to keep in sync.

## Key Abstractions

**Workspace (Write-Through Session):**
- Purpose: Eliminates dual-write pattern by coordinating repository and graph
- Examples: `internal/workspace/workspace.go`, `internal/workspace/document.go` (read methods), `internal/workspace/document.go` (write methods)
- Pattern: Methods like CreateEntity return *model.Entity and maintain graph consistency

**Repository Store Interface:**
- Purpose: Allow different storage backends (filesystem, database, API)
- Examples: `internal/repository/repository.go` defines Store interface
- Pattern: Repository implements Store for filesystem; tests use mock Store implementations

**Metamodel Resolver:**
- Purpose: Dynamically resolve entity type names (handle aliases), validate relations, compute file paths
- Examples: `internal/metamodel/metamodel.go` (GetEntityDef, ResolveAlias, ValidateRelation)
- Pattern: Handlers pass metamodel to repository for path computation

**Transaction Coordinator:**
- Purpose: Batch multiple writes and apply atomically using rename-based commit
- Examples: `internal/repository/transaction.go` (Transaction method, transaction struct)
- Pattern: fn(tx) stages writes to .new files, tx.commit() renames atomically

**Event Broker (SSE):**
- Purpose: Deliver live-reload notifications to connected web clients
- Examples: `internal/dataentry/events.go` (eventBroker struct, handleSSE)
- Pattern: Handlers receive SSE events when workspace syncs from file changes

## Entry Points

**CLI Entry Point:**
- Location: `cmd/rela/main.go`
- Triggers: User runs `rela` command
- Responsibilities: Import `internal/cli` package and call `Execute()` which runs Cobra command dispatcher

**CLI Root Command:**
- Location: `internal/cli/root.go`
- Triggers: CLI bootstrap
- Responsibilities:
  - Define global flags (--project, --output, --verbose)
  - Implement PersistentPreRunE: discovers project, loads metamodel, syncs graph, creates workspace
  - Share state (ws, meta, projectCtx, out) across all commands

**Server Entry Point:**
- Location: `cmd/rela-server/main.go`
- Triggers: User runs `rela-server [-project .]` or Wails desktop app
- Responsibilities:
  - Parse flags (-project, -port)
  - Create repository via project discovery
  - Initialize workspace
  - Create dataentry App and start file watcher
  - Start HTTP server on port with App.NewRouter()

**Web Request Entry Points (Data Entry App):**
- Location: `internal/dataentry/router.go` (NewRouter method)
- Triggers: HTTP requests to various paths
- Responsibilities:
  - Route to handlers (handleList, handleEntity, handleCreate, handleUpdate, etc.)
  - All handlers wrapped with reload-lock middleware (RWMutex) except SSE
  - SSE endpoint (handleSSE) excluded from lock since it holds connection open

**MCP Server Entry Point:**
- Location: `internal/mcp/server.go`
- Triggers: `rela mcp` command or IDE plugin initialization
- Responsibilities:
  - Create MCP server wrapping workspace
  - Register tools (23 total: entity/relation CRUD, analysis, export)
  - Register resources (metamodel, entity, relation)
  - Register prompts (analyze-traceability, review-orphans, etc.)
  - Start file watcher for live notification of changes
  - Serve on stdio

## Error Handling

**Strategy:** Wrapped errors with type assertions for specific error handling; defined error variables for sentinel comparison

**Patterns:**

- **Sentinel errors** (`internal/errors/errors.go`): `ErrNotFound`, `ErrInvalidType`, `ErrValidation` for comparison with `errors.Is()`
- **Custom error types** with Unwrap(): `EntityNotFoundError`, `EntityTypeNotFoundError`, `ValidationError` for detailed context
- **String wrapping**: `fmt.Errorf("context: %w", err)` to add context while preserving original error
- **Handler error responses**: HTTP 400/404/500 with error messages rendered in templates or JSON
- **Configuration validation errors** (`dataentry.ConfigValidationError`): Slice of strings for multiple validation failures

Example from `cmd/rela-server/main.go`:
```go
if errors.As(err, &configErr) {
    for _, e := range configErr.Errors {
        fmt.Fprintf(os.Stderr, "  - %s\n", e)
    }
    os.Exit(1)
}
```

## Cross-Cutting Concerns

**Logging:**
- Strategy: Use Go standard `log` package; MCP server uses custom logger with stderr prefix
- Pattern: Log at startup (CLI/server initialization), errors, and significant state changes (file watcher, sync)

**Validation:**
- Strategy: Validate on read (metamodel schema), on write (entity type, relation type), and on migration detection
- Pattern: Metamodel defines validation rules; repository validates before write; dataentryconfig validates on load

**Authentication:**
- Strategy: Not implemented at application level; relies on deployment environment (reverse proxy, Wails native)
- Pattern: Data entry web app has no auth middleware; MCP server assumes trusted client (stdio)

**Concurrency:**
- Strategy: RWMutex protecting reloadable state in App struct; thread-safe Graph with RWMutex; atomic file writes via SafeFS
- Pattern: Handlers acquire read lock; reload acquires write lock; graph operations are serialized

---

*Architecture analysis: 2026-03-19*
