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
| `internal/dataentry` | Config-driven data entry web app (Go API handlers serving Vue 3 SPA in `frontend/`) |
| `internal/model`     | Core types: `Entity`, `Relation`, `Status`              |
| `internal/graph`     | In-memory graph with adjacency lists, tracing, analysis |
| `internal/metamodel` | Schema loading, validation, custom types                |
| `internal/markdown`  | Parse/write entities and relations from markdown files  |
| `internal/project`   | Project discovery, paths (`Context`)                    |
| `internal/output`    | CLI output formatting (table, JSON)                     |
| `internal/filter`    | Entity filtering by properties                          |
| `internal/mcp`       | MCP server: tools, resources, prompts, file watcher     |
| `internal/migration` | Schema migration system for project files               |
| `internal/scheduler` | Sequential Lua script scheduler with missed-run detection |

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

- **23 tools**: Entity/relation CRUD, graph tracing, analysis, schema introspection, export
- **3 resources**: `rela://metamodel`, `rela://entity/{type}/{id}`, `rela://relation/{from}/{type}/{to}`
- **4 prompts**: `analyze-traceability`, `review-orphans`, `summarize-project`, `review-entity`
- **File watcher**: Watches entities/, relations/, and metamodel.yaml with 200ms debounce

The `rela mcp` command handles its own initialization (project discovery, metamodel loading, graph
sync) independently from the standard CLI state setup in `PersistentPreRunE`.

### CLI State Initialization

`internal/cli/root.go` sets up shared state in `PersistentPreRunE`:

1. Discover project root (find `metamodel.yaml`)
2. Load metamodel
3. Initialize graph (from cache or by syncing markdown files)

All commands share `projectCtx`, `meta`, `g` (graph), and `out` (output writer).

Commands that manage their own initialization (or don't need a project) annotate themselves
with `skipProjectDiscovery: "true"` in their Cobra `Annotations` map. This includes:
`init`, `version`, `completion`, `migrate`, `mcp`, `validate`, `flow`, `scheduler`.

### Scheduled Tasks

The `rela scheduler` command starts a long-running process that executes Lua scripts on
recurring schedules defined in `schedules.yaml`. Like `rela mcp`, it handles its own
workspace initialization independently from `PersistentPreRunE`.

**Configuration** (`schedules.yaml` in project root):

```yaml
tasks:
  - name: daily-report
    script: reports/daily.lua
    every: day
  - name: weekly-review
    script: checks/weekly.lua
    every: week
  - name: quick-check
    script: checks/orphans.lua
    every: 30m
```

**Schedule values:**

| Value        | Meaning                                              |
| ------------ | ---------------------------------------------------- |
| `day`        | Once per day (after midnight local time)              |
| `monday`     | Once per week on Mondays (any weekday name works)     |
| `friday`     | Once per week on Fridays                              |
| `week`       | Alias for `monday`                                    |
| `30m`, `2h`  | Fixed interval (any Go duration)                      |
| `15`         | Bare number interpreted as minutes                    |

**Architecture:**

- Single-threaded sequential loop — tasks execute one at a time in config order
- Workspace is synced before each task execution for fresh graph state
- Scripts have the same capabilities as `rela script` (entity CRUD, graph queries, AI)
- State persisted in `.rela/scheduler-state.json` (last-run timestamps per task)
- On startup, tasks that missed their window execute immediately
- Graceful shutdown on SIGINT/SIGTERM

**Key types** (`internal/scheduler/`):

- `Config` / `TaskConfig`: YAML config with name, script path, schedule
- `Schedule`: Parsed schedule (day/week/interval) with `IsDue(lastRun, now)` method
- `Scheduler`: Main loop — `Run(ctx)` blocks until context cancellation
- `State`: JSON-serialized last-run timestamps

## AI Integration

The `internal/ai` package provides LLM access via OpenAI-compatible providers
(OpenAI, Anthropic via compat layer, Ollama, LM Studio, Groq, apfel, etc.).
It is exposed to Lua scripts as a top-level `ai` global.

### Configuration

User config lives in `.rela/ai.yaml` (gitignored, per-user):

```yaml
base_url: http://127.0.0.1:11434/v1   # required, must include scheme
model: gemma3:12b                      # required, default model
embedding_model: nomic-embed-text      # optional; falls back to model
api_key_env: OPENAI_API_KEY            # optional; absent = no auth header
timeout_seconds: 60                    # optional, default 30
```

`api_key_env` is **optional**. When absent, no `Authorization` header is sent —
this supports local providers like ollama, apfel, and LM Studio that run without
authentication. When set, the named env var is read at request time (not at
startup), so commands that don't use AI never fail because the env var is unset.

### Lua API

```lua
-- Full form
local result, err = ai.chat({
  messages = {
    {role = "system", content = "You are concise."},
    {role = "user",   content = "What is 2+2?"},
  },
  model = "gemma3:12b",   -- optional, falls back to .rela/ai.yaml
  temperature = 0,        -- optional; 0 is sent distinctly from "unset"
  max_tokens = 50,        -- optional
})

-- Convenience: single user message, returns just the content string
local text, err = ai.complete("Summarize: " .. content)

-- Embeddings: single text or batch
local vecs, err = ai.embed("hello world")          -- returns {{0.1, 0.2, ...}}
local vecs, err = ai.embed({"first", "second"})    -- returns {{...}, {...}}
local vecs, err = ai.embed("text", {model = "nomic-embed-text"})  -- model override
```

**ai.chat** returns a table `{content, model, finish_reason, usage}` with
flat fields. `usage` is a sub-table `{prompt_tokens, completion_tokens, total_tokens}`.

**ai.embed** always returns an array of arrays (one vector per input), even for
a single string input. Vectors are float64. Batch input is limited to 2048 texts.
Empty strings and empty tables raise a programming error.

On failure: `err` is a typed table `{kind, status, message, retry_after}` with
stable `kind` values:

| `err.kind` | When |
|---|---|
| `not_configured` | No `.rela/ai.yaml` or it failed to load |
| `auth` | API key missing/invalid; HTTP 401/403 |
| `bad_request` | HTTP 400/4xx; unknown model, unsupported parameter |
| `rate_limited` | HTTP 429; `err.retry_after` populated when server provides Retry-After |
| `server_error` | HTTP 5xx |
| `timeout` | Request exceeded its deadline |
| `network` | DNS, connection refused, TLS, etc. |
| `bad_response` | Non-JSON, malformed JSON, missing choices, unrecognized content shape |
| `streaming_unsupported` | Provider returned SSE despite `stream: false` |

**Convention deviation**: `ai.chat`, `ai.complete`, `ai.embed`, and the `http.*`
module return `(nil, err_table)` for runtime failures. All other rela bindings
raise via `RaiseError`. The deviation is deliberate — network-bound calls have
expected failure modes, and scripts should handle them inline rather than wrap
every call in `pcall`. Programming errors (wrong arg type, empty input) still
raise. See `internal/lua/ai.go` top-of-file comment for the full rationale.

### Security: New threat surface

Adding AI to the Lua sandbox introduces a **new threat class**: a malicious or
compromised Lua script can now silently exfiltrate every entity in the project
to the user's *own* legitimate provider via
`ai.chat({messages = {{role="user", content=entity_dump}}})`. The data lands in
the provider's logs, possibly in training data, possibly readable by junior
staff, possibly billed to the user. The script needs no malicious config and no
filesystem write — it uses the user's own working setup.

**Mitigations in place:**

- Operational logging (debug/info/warn) makes unusual call patterns visible
- API is opt-in: requires `.rela/ai.yaml` to exist
- API key is read at call time and never logged or echoed in errors (`redactKey` helper + table-driven leak test)
- Config rejects URLs with embedded credentials (`https://user:pass@host`)
- Response body is capped at 10 MiB to prevent OOM

**Treat Lua scripts as trusted code.** The `rela.write_file`, `ai.chat`, and
`http.*` capabilities together mean a malicious script can do real damage.
Don't run Lua scripts you don't trust.

### HTTP API

The `http` module provides HTTP client capabilities for Lua scripts to call
external APIs. No configuration is needed — unlike `ai.*`, the HTTP module
works out of the box.

```lua
-- Full form
local resp, err = http.request({
  url     = "https://api.example.com/data",
  method  = "POST",                          -- optional, default GET
  headers = {["Content-Type"] = "application/json"},
  body    = http.json_encode({key = "value"}),
  timeout = 10,                              -- optional, seconds
})

-- Convenience methods
local resp, err = http.get(url, opts?)
local resp, err = http.post(url, body, opts?)
local resp, err = http.put(url, body, opts?)
local resp, err = http.patch(url, body, opts?)
local resp, err = http.delete(url, opts?)

-- JSON helpers
local str = http.json_encode(value)             -- Lua value → JSON string
local val, err = http.json_decode(json_string)  -- JSON string → Lua value
```

On success: `resp` is a table `{status_code, status, headers, body}`.
`headers` is a flat table with lowercase keys (first value wins for
multi-value headers).

On failure: `err` is a typed table `{kind, status, message, details}`:

| `err.kind` | When |
|---|---|
| `timeout` | Request exceeded deadline (per-request or 30s default) |
| `canceled` | Request was canceled (e.g., runtime shutting down) |
| `network` | DNS, connection refused, TLS, etc. |
| `bad_response` | Response body exceeded 10 MiB limit |

Redirects are NOT followed — the 3xx response is returned directly so scripts
can handle redirects explicitly.

`http.json_decode` returns `(nil, err_table)` for invalid JSON (expected
runtime failure from external data). Wrong argument types raise via
`RaiseError` (programming error).

### Operational logging

Every AI request emits structured log lines via `slog`:

- `ai request start base_url=... model=... messages=N` (debug)
- `ai request ok status=200 model=... latency_ms=... prompt_tokens=... completion_tokens=... total_tokens=...` (info)
- `ai embed start base_url=... model=... inputs=N` (debug)
- `ai embed ok status=200 model=... latency_ms=... vectors=N prompt_tokens=... total_tokens=...` (info)
- `ai request failed kind=... status=... latency_ms=... message=...` (warn)

No headers, no API keys, no message content are ever logged.

### Architecture

```text
internal/ai/
  config.go        Config struct, LoadConfig (ErrConfigNotFound on missing file)
  errors.go        ErrKind enum, *Error type, classify(), redaction
  provider.go      Provider interface, Chat/Embed requests and responses
  openai.go        Chat HTTP implementation, shared request/response infrastructure
  openai_embed.go  Embed HTTP implementation (wire types, response parsing)
  loader.go        LoadProvider helper for entry-point wiring
  redact.go        redactKey(s, key) helper
```

The `Provider` interface aggregates `Chat` and `Embed`. The Lua runtime takes a
single `ai.Provider` via `lua.WithAIProvider(p)`, so adding capabilities does not require
parallel wiring paths.

Four entry points wire AI into the Lua runtime: `internal/cli/script.go`,
`internal/cli/flow.go`, `internal/script/executor.go`, `internal/mcp/tools_lua.go`.
Each calls `ai.LoadProvider(.rela_dir)` and passes the result to `lua.New` via
`WithAIProvider`. Missing or malformed config silently disables AI for that
runtime; the Lua bindings return `not_configured` at call time.

**`internal/validation/lua.go` is intentionally NOT wired with AI.** A validation
rule that calls `ai.chat` would hit the provider on every entity on every
`analyze` run with no quota, no kill switch, and no cost warning. The 5-second
validation timeout would also silently clip slow calls. AI-powered validation
rules need their own design (per-rule opt-in, cost guardrails, longer per-rule
budget) and are tracked as a follow-up.

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

// coverage-ignore: requires external graphviz installation
func renderWithGraphviz() error {
    // ...
}
```

Valid reasons for `coverage-ignore`:

- Main/entry point functions (better tested via integration tests)
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

- Test files exempt from dupl, funlen, magic numbers
- Cobra `cmd`/`args` unused parameters are allowed
- Line length limit: 120 chars

## Project Files

```text
metamodel.yaml              # Entity/relation schema
schedules.yaml              # Optional: scheduled task definitions for `rela scheduler`
entities/<type>/            # Markdown entity files by type
relations/                  # Markdown relation files (FROM--type--TO.md)
templates/entities/<type>.md  # Optional: entity templates for defaults
templates/relations/<type>.md # Optional: relation templates for defaults
.rela/cache.json            # Graph cache (gitignored)
.rela/user-defaults.yaml    # User-specific default values (gitignored)
.rela/scheduler-state.json  # Scheduler last-run timestamps (gitignored)
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

<!-- @managed: claude-workflow start -->

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

### Workflow Checklists

Tickets and bugs use workflow checklists to ensure thorough planning, execution, and review.
Each phase has a dedicated checklist entity with standard items from templates.

**Ticket Workflow:**

```text
backlog → ready → planning → in-progress → review → done
                     │            │           │
                     ▼            ▼           ▼
              planning-      implementation-  review-checklist
              checklist      checklist        (+ docs-checklist
                 │                            for enhancements)
                 ▼
           /design-review
           (before impl)
```

**Bug Workflow:**

```text
backlog → ready → analyzing → in-progress → review → done
                     │            │           │
                     ▼            ▼           ▼
              bug-analysis-  implementation-  review-checklist
              checklist      checklist
```

**Checklist Types:**

| Checklist | Purpose | Required For |
|-----------|---------|--------------|
| `planning-checklist` | Understanding, research, approach, security, risk assessment | Tickets entering `in-progress` |
| `bug-analysis-checklist` | Reproduction, root cause, fix planning | Bugs entering `in-progress` |
| `implementation-checklist` | Development, quality checks | Tickets/bugs entering `review` |
| `review-checklist` | Automated checks, code review, verification | Tickets/bugs entering `done` |
| `docs-checklist` | Code docs, project docs, external docs | Enhancement/docs tickets entering `done` |

**Review Commands:**

| Command | When to Use | Creates |
|---------|-------------|---------|
| `/design-review` | After planning, before implementation | `review-response` entities for design issues |
| `/code-review` | During review phase, after implementation | `review-response` entities for code issues |

**Agent Workflow for Tickets:**

Checklists are **automatically created** when tickets/bugs transition to specific statuses.
The `create_entity` automation with `if_exists: skip` ensures no duplicates.

1. **Start Planning** (status: `planning`)
   - Planning checklist is auto-created and linked via `has-planning`
   - Work through checklist items: understanding, approach, security, test plan
   - Run `/design-review` to catch issues before implementation
   - Address all critical/significant design findings
   - Mark checklist `status=done` when complete

2. **Start Implementation** (status: `in-progress`)
   - Implementation checklist is auto-created and linked via `has-implementation`
   - Work through development and quality items

3. **Start Review** (status: `review`)
   - Review checklist is auto-created and linked via `has-review`
   - Run `/code-review` to perform thorough code review
   - Address all critical/significant code review findings
   - If enhancement or docs ticket, manually create `docs-checklist`
   - Complete all checks before marking done

4. **Create PR** (before `done`)
   - Run `/pr` to create PR and monitor CI until all checks pass
   - Fixes any CI failures (lint, test, coverage) automatically
   - Document PR URL in review-checklist

5. **Complete** (status: `done`)
   - All linked checklists must have `status=done`
   - All checklist items must be checked or skipped with reason
   - PR merged or ready to merge

**Bug Workflow Automations:**

- `analyzing` → auto-creates `bug-analysis-checklist` via `has-bug-analysis`
- `in-progress` → auto-creates `implementation-checklist` via `has-implementation`
- `review` → auto-creates `review-checklist` via `has-review`

**Skipping Checklist Items:**

When an item doesn't apply, use strikethrough with a reason in parentheses:

```markdown
- [x] ~~API docs updated~~ (N/A: no API changes)
- [x] ~~Performance check~~ (N/A: documentation-only change)
```

Items without reasons will fail validation.

### Review Response Protocol

**Triggering Code Review:**

When a ticket/bug enters `review` status, run the `/code-review` command. This invokes the
cranky-code-reviewer agent to perform a thorough code review and automatically creates
`review-response` entities for each finding.

Alternatively, invoke the cranky-code-reviewer agent directly for ad-hoc reviews.

**Creating Review Responses:**

For each finding from code review:

1. Create a `review-response` entity with:
   - `title`: Brief description of the finding
   - `finding`: Full description of the issue
   - `severity`: `critical` | `significant` | `minor` | `nit`
   - `status`: `open`
2. Link to ticket/bug via `has-review-response` relation

**Addressing Review Responses:**

| Severity | Required Action |
|----------|-----------------|
| critical | MUST be fixed before done |
| significant | MUST be fixed before done |
| minor | Should fix, can defer with reason |
| nit | Optional, can wont-fix with reason |

When addressing a finding:

- Fix the issue in code
- Update status to `addressed`
- Document the `resolution` (how it was fixed)

When not addressing:

- Set status to `wont-fix` or `deferred`
- Document the `reason` (justification required)

**Validation Gates:**

Tickets/bugs cannot be marked `done` if they have:

- Open critical review responses
- Open significant review responses

Minor/nit findings may remain open with warnings.

### Automation Actions

The automation engine supports these action types in `metamodel.yaml`:

**set**: Set a property value on the triggering entity

```yaml
automations:
  - name: set-date-on-done
    on:
      entity: [ticket]
      property: status
      becomes: done
    do:
      - set: completed_at
        value: "{{today}}"
```

**create_relation**: Create a relation from the triggering entity

```yaml
automations:
  - name: link-on-assignment
    on:
      entity: [ticket]
      property: assignee
    do:
      - create_relation:
          relation: assigned-to
          to: "{{new.assignee}}"
```

**create_entity**: Create a new entity (with optional relation to trigger)

```yaml
automations:
  - name: create-checklist-on-ready
    on:
      entity: [ticket]
      property: status
      becomes: ready
    do:
      - create_entity:
          type: planning-checklist
          properties:
            title: "Planning: {{new.title}}"
            status: open
          relation: has-planning    # Creates relation FROM trigger TO new entity
          if_exists: skip           # What to do if relation already exists
```

**if_exists options** (for `create_entity` with `relation`):

| Value | Behavior |
|-------|----------|
| `skip` | Skip creation if relation to same entity type exists (default) |
| `error` | Return error if relation to same entity type exists |
| `replace` | Delete existing target entity and create new one |

The `if_exists` check uses the relation to detect duplicates: if the trigger entity
already has a relation of the specified type pointing to an entity of the same type
being created, the action is considered a duplicate.

### Interpolation Syntax

Automation properties support template interpolation:

| Pattern | Description |
|---------|-------------|
| `{{new.property}}` | Property from new/current entity |
| `{{entity.id}}` | ID of the triggering entity |
| `{{today}}` | Current date (YYYY-MM-DD) |

Common mistake: `{{entity.title}}` is WRONG, use `{{new.title}}` instead.

### Test Writing Best Practices

Follow these patterns to make tests clearer and more maintainable.

**Use Fluent Test Builders:**

Create fluent builder APIs for test fixtures. Only specify values that matter for the specific
test - let builders handle defaults, auto-generate IDs, and fill required fields with random data.

```python
# BAD - verbose, specifies irrelevant details
config = AutomationConfig(
    name="test-automation",
    trigger=Trigger(entity_types=["ticket"], event="created"),
    actions=[Action(type="set", property="status", value="open")]
)
entity = Entity(id="T-001", type="ticket", properties={})

# GOOD - fluent, only specifies what matters
config = automation().on_create("ticket").set("status", "open").build()
entity = entity("ticket").build()  # ID auto-generated
```

**Auto-Generate Identifiers:**

Builders should auto-generate IDs, names, and other identifiers when not explicitly set.
This catches bugs where code accidentally depends on specific values and reduces test noise.

```python
# BAD - hardcoded ID that test doesn't care about
user = create_user(id="user-123", name="Test User")

# GOOD - auto-generated, test doesn't depend on specific value
user = user_builder().with_name("Test User").build()
```

**Minimize Test Setup:**

If test setup takes more than a few lines, create a builder or helper. Common patterns to simplify:

| Verbose Pattern | Fluent Alternative |
|-----------------|-------------------|
| Nested object construction | Chained builder methods |
| Multiple required fields | Sensible defaults in builder |
| Repeated boilerplate | Shared test fixtures |
| Complex state setup | Purpose-named factory methods |

**Avoid Hardcoded Values in Assertions:**

Don't compare against hardcoded strings when the object is in scope:

```python
# BAD - couples test to specific value
entity = create_entity(id="T-001")
assert relation.source == "T-001"

# GOOD - uses object reference
entity = create_entity()
assert relation.source == entity.id
```

For interpolated values, construct the expected value from the object:

```python
# BAD
assert result.title == "Checklist for T-001"

# GOOD
assert result.title == "Checklist for " + entity.id
```

For preserved properties, compare against the original object:

```python
# BAD
assert updated.title == "Original Title"

# GOOD
assert updated.title == original.title
```

**When Hardcoded Values ARE Appropriate:**

- **Ordering tests**: Verifying sort order requires deterministic values
- **Parse/read tests**: Verifying parser reads specific values from fixtures
- **Trigger values**: Testing rules that trigger on specific values (e.g., status="done")
- **Format validation**: Testing specific formats, patterns, or error messages

**Use Local Variables for Repeated Values:**

When values are passed to helpers and then asserted, extract to variables:

```python
# BAD - duplicated string
create_entity(id="REQ-001")
assert relation.source == "REQ-001"

# GOOD - single source of truth
req_id = "REQ-001"
create_entity(id=req_id)
assert relation.source == req_id
```

**Clone for Variations:**

When testing state changes, clone the original rather than creating two separate objects:

```python
# BAD - duplicates setup, easy to get out of sync
old_entity = entity("ticket").with_status("backlog").with_title("Fix bug").build()
new_entity = entity("ticket").with_status("done").with_title("Fix bug").build()

# GOOD - clone ensures consistency
old_entity = entity("ticket").with_status("backlog").with_title("Fix bug").build()
new_entity = old_entity.clone()
new_entity.status = "done"
```

**Benefits:**

1. **Random test data**: Catches bugs where code accidentally depends on specific values
2. **Clearer intent**: Only explicitly set values that matter for the test
3. **Less boilerplate**: Builders handle defaults and required fields
4. **Easier refactoring**: Change formats/schemas without updating every test
5. **Better coverage**: Random values may catch edge cases hardcoded values miss
<!-- @managed: claude-workflow end -->
