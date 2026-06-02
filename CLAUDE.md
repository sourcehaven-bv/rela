# CLAUDE.md

## Rules for new code

- **Define interfaces at the call site, not next to the implementation.**
  Producer-side interfaces couple consumers to every method the producer
  exposes. Each consumer declares the minimum interface it needs (one to
  three methods). When a callback would create a constructor cycle, the
  consumer defines the narrow interface and the wiring site supplies it —
  see `docs/architecture/consumer-side-interfaces.md` and the godoc on
  `autocascade.Host`, `mcp.Services`, `scheduler.WorkspaceProvider`.
- **Capability bundles, not service locators.** When a subsystem needs
  several collaborators, group them in a purpose-specific struct (see
  `internal/lua/deps.go` with `ReadDeps` / `WriteDeps`), split by read vs.
  write so read-only code can't accidentally mutate state. A scoped
  consumer-side `Services` interface is fine (see `internal/mcp/server.go`);
  a cross-subsystem grab-bag is not.
- **No repository or transaction abstractions.** Depend directly on
  `store.Store`, `tracer.Tracer`, `search.Searcher`,
  `entitymanager.EntityManager`. The old `repo` and `tx` layers are gone
  — do not reintroduce equivalents.
- **Capture state once per operation.** Call `ws.Snapshot()` (or the
  equivalent `appState.Load()`) at the top of every handler, command, MCP
  tool, or observer; reuse the returned value for every read in that
  operation. Do not call `ws.Graph()` / `ws.Meta()` repeatedly — multiple
  loads against the underlying `atomic.Pointer` can observe different
  snapshots if a reload lands between them.
- **Don't leak storage or parsing types via return values.** A function
  that returns `*markdown.Document`, `*graph.Graph`, `interface{}`, or any
  type whose package the caller wouldn't otherwise need pulls the
  implementation into every consumer. Return value types or
  domain-package DTOs (`entity.Entity`, `entity.Relation`, `tracer.Result`).
  If you reach for `interface{}` plus a type assertion as a back-channel,
  define a typed dependency instead.
- **Split state-publish from write-serialize.** Use `atomic.Pointer[State]`
  for publishing a new state snapshot (no reader lock, no torn reads) and
  a separate `sync.Mutex` for serializing writers. Do not combine both
  responsibilities into a single `sync.RWMutex` — the lock-upgrade dance
  (`RUnlock → Lock → defer(Unlock → RLock)`) is the symptom, not the fix.
- **Constructors reject nil required fields.** A `New*` function with
  required collaborators returns `error` and validates them up front.
  Never substitute a no-op or sentinel implementation silently — that
  defers the failure to a downstream symptom that is much harder to
  diagnose.
- **Boundaries are enforced.** `just arch-lint` checks package import
  rules; run it before PR.

### Don't do this

- **Don't import `internal/graph` or `internal/model`** — both deleted.
  Use `internal/entity`, `internal/store`, `internal/tracer`.
- **Don't add a cross-subsystem service locator** (à la the removed
  `lua.Services`). Use `ReadDeps` / `WriteDeps` or a scoped consumer-side
  interface.
- **Don't call `ai.LoadProvider` directly from a new entry point.** Go
  through `script.NewWriterRuntime`, which calls `lua.LoadContextOptions`.
- **Don't wire AI into the validation path** — per-entity cost blowup
  with no quota. See `internal/ai/` docs for the rationale.
- **Don't reintroduce `internal/workspace`.** It was the legacy
  god-object aggregate; deleted in the workspace-decomposition arc.
  New code wires services individually via `appbuild.Discover` /
  `appbuild.New` or takes focused interfaces at the call site.
- **Don't run user-supplied Lua on the read path.** ACL gates evaluate
  against declarative policy + the graph; Lua participates only at write
  time. See `internal/entitymanager/CLAUDE.md`.

### Subsystem-specific rules (nested CLAUDE.md / godoc)

- **Writes, audit, ACL** → `internal/entitymanager/CLAUDE.md`. All writes
  go through `entitymanager.Manager`; do not write to `store.Store`
  directly from a write path.
- **Data-entry API + `_actions` affordances + write-validation policy** →
  `internal/dataentry/CLAUDE.md`.
- **Vue SPA build/test/architecture** → `frontend/CLAUDE.md`.
- **E2E tests** → `e2e/tests/AGENTS.md`.

## Architecture

rela is a schema-driven entity-graph platform. You define the shape of your
domain in a YAML metamodel (entity types, relation types, properties,
validation rules); rela gives you typed entities, typed relations, and tools
to query / validate / analyze / present the graph. Data is stored as markdown
files with YAML frontmatter.

Traceability (requirements → decisions → components) is one common use case,
not the identity. Other in-tree uses: ISO 27001 ISMS, project management,
DevOps runbooks, issue/ticket tracking (rela dogfoods itself — see `tickets/`),
documentation mirrors (`docs-project/`). Anything with typed entities and
relations fits.

```text
metamodel.yaml → Metamodel (entity types, relations, properties)
                     ↓
entities/*.md  → entity.Entity  ↘
                                 store.Store → tracer.Tracer  (pure reader)
relations/*.md → entity.Relation ↗          → search.Searcher (EntityObserver)
                                            → entitymanager.EntityManager
                                              (writes + automations + validation)
```

The store is the source of truth. `search` maintains a derived index as a
`store.EntityObserver`. `tracer` is a pure reader — no subscription, no
derived state. `entitymanager` is the "human intent" write path that runs
automations and validation on top of the store.

Write-path rules — validation policy (400/422/200-with-warnings), the
audit log, and ACL — live in the nested files
`internal/dataentry/CLAUDE.md` and `internal/entitymanager/CLAUDE.md`.

### Packages

Entry points: `cmd/rela`, `cmd/rela-server`, `cmd/rela-desktop`.

Domain and storage:

| Package                  | Purpose                                                   |
| ------------------------ | --------------------------------------------------------- |
| `internal/entity`        | Domain types: `Entity`, `Relation` (no storage metadata)  |
| `internal/metamodel`     | Schema: entity types, relations, properties, validation   |
| `internal/store`         | Storage abstraction — CRUD + events; `fsstore`/`memstore`/`pgstore` |
| `internal/tracer`        | Pure-reader graph traversal (trace, path, orphans, cycles)|
| `internal/search`        | Full-text + structured search (bleve + linear)            |
| `internal/entitymanager` | Write path: automations, validation, audit, policy        |
| `internal/audit`         | Append-only JSONL audit log of every successful write     |
| `internal/principal`     | Identity attribution (`Principal{User, Tool}`) on ctx     |
| `internal/validator`     | Validation engine invoked by entitymanager                |
| `internal/markdown`      | Parse/write entity and relation markdown                  |
| `internal/project`       | Project discovery, paths (`Context`)                      |
| `internal/appbuild`      | Wiring facade — constructs the focused services bundle    |

Subsystems (see each package's doc comment for details):

| Package               | Purpose                                                        |
| --------------------- | -------------------------------------------------------------- |
| `internal/cli`        | Cobra commands                                                 |
| `internal/mcp`        | MCP server over stdio — tools, resources, prompts, watcher    |
| `internal/dataentry`  | Data entry web app (Go API + Vue 3 SPA in `frontend/`)         |
| `internal/scheduler`  | Sequential Lua script scheduler (`rela scheduler`)             |
| `internal/lua`        | Lua runtime + bindings (`ReadDeps`, `WriteDeps`)               |
| `internal/script`     | Script execution helpers that wrap `lua` with project context  |
| `internal/automation` | Automation engine invoked by `entitymanager`                   |
| `internal/autocascade`| Cascade orchestration (runs automation side-effects)           |
| `internal/ai`         | OpenAI-compatible LLM provider (used from Lua)                 |
| `internal/migration`  | Schema migrations for project YAML files                       |

Other packages under `internal/` are self-descriptive — ls the tree.

### Storage backends & build tags

The storage + search backend is chosen at compile time by Go build tags.
The composition root has one `New` recipe per scenario over shared
`prepare()`/`assemble()` helpers — see `internal/appbuild/appbuild_{fs,memory,postgres}.go`
and the matching `internal/cli/mcp_wiring_{fs,memory,postgres}.go`:

| Build tag        | Store      | Search                | Binaries                          |
| ---------------- | ---------- | --------------------- | --------------------------------- |
| _(none, default)_| `fsstore`  | in-memory bleve       | `rela`, `rela-server`             |
| `memorybackend`  | `memstore` | `LinearSearch`        | (tests / experiments; no bleve)   |
| `postgres`       | `pgstore`  | PostgreSQL (`pg_trgm` + tsvector) | `rela-postgres`, `rela-server-postgres` |

Rules when touching this:

- **The `postgres` build must not link bleve; the default build must not
  link pgx.** CI asserts both via `go list -deps` (the `postgres` job in
  `ci.yml`). Keep backend-specific imports inside the tagged recipe files.
- **`pgstore.New(db DBTX)` takes an injected pgx pool**, not a DSN. The
  postgres recipe builds one pool, runs `pgstore.Migrate`, and shares it
  between the store and the in-DB search backend. appbuild owns/closes the
  pool; `store.Close()` only tears down the watcher.
- **Build-agnostic wiring lives in `prepare`/`assemble`, never in a recipe.**
  A recipe may choose and order backend steps; if logic would be copy-pasted
  between recipes, it belongs in a shared helper. This is what keeps the three
  recipes from drifting (and where future per-backend audit/ACL variation goes).
- **The metamodel is always read from disk**, even in the postgres build —
  `metamodel.yaml`, `templates/`, `.rela/` stay on the filesystem; PostgreSQL
  backs entities/relations/attachments/search only. A postgres deployment
  still needs a `--project` dir.
- **Single-writer scope.** The postgres watcher is in-process only (mirrors
  fsstore/memstore). Rows carry `created_at`/`updated_at`/`seq` so a future
  cross-process change feed is additive (no migration). Don't wire a second
  writer against one database yet.
- DSN resolution: `--database-url` flag (`rela`, `rela-server`) > `RELA_DATABASE_URL`
  env. Threaded via `appbuild.Config.DatabaseURL` / `appbuild.WithDatabaseURL`.
- A new `store.Store` implementation must pass `internal/store/storetest`
  (`RunAll` + the fuzz functions). pgstore's suite is DB-gated on
  `RELA_TEST_DATABASE_URL` (skips when unset). Run it with `just test-postgres`.

## Tests

- Prefer table-driven tests with `t.Run(tc.name, ...)` subtests.
- Use `t.Helper()` on assertion helpers.
- `internal/store/storetest` provides the store conformance harness — any
  new `store.Store` implementation must pass it.
- Race detector is on in CI; don't add `//go:build !race` tags.

## Coverage

Go: `go-test-coverage` enforces **package floor thresholds** (no ratchet);
minimums live in `.testcoverage.yml`. Coverage within the floor is free to
move up or down — floors exist to catch "new untested package added" and
"core package silently lost its tests." Frontend uses a separate per-file
ratchet at 100%.

- Run locally: `just coverage-check`, `just coverage-html`.
- When a floor fails, add tests — don't lower the threshold without a reason.
- Use `// coverage-ignore: <reason>` sparingly, only for genuinely
  untestable code (main functions, external-tool dependencies,
  OS-specific paths). Reason is required.

## Lint

golangci-lint with project rules. Test files exempt from `dupl`, `funlen`,
magic numbers. Cobra `cmd`/`args` unused parameters allowed. Line length: 120.

## Security

`govulncheck` runs on every PR touching `go.mod` / `go.sum` (the `vulncheck`
job in `ci.yml`) and weekly from `security.yml`. Known-unfixable vulns are
filtered via `scripts/govulncheck-filtered.sh` — keep `IGNORED_OSVS` in sync
with `scripts/govulncheck-fixable.sh`. Run locally: `just govulncheck`.

## Commands

Read the `justfile` for the full set. The non-obvious ones: `just arch-lint`
(package boundary check), `just ci` (full pipeline), `just dev` (data-entry
server locally), `just coverage-check`. `go test -run TestName ./...` for a
single test.

## Project files

```text
metamodel.yaml                  # Entity/relation schema
schedules.yaml                  # Optional: schedules for `rela scheduler`
entities/<type>/                # Markdown entity files by type
relations/                      # Markdown relation files (FROM--type--TO.md)
templates/entities/<type>.md    # Optional: entity templates for defaults
templates/relations/<type>.md   # Optional: relation templates for defaults
.rela/user-defaults.yaml        # Per-user defaults (gitignored)
.rela/scheduler-state.json      # Scheduler last-run timestamps (gitignored)
```

## Working documents

Anything temporary — designs, tickets, QA notes, scratch — goes in
`.ignored/` (gitignored). Do not commit these.

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
| research | `researches` → concept (min 1) |

### Research Documents

For larger features, run `/research <topic>` before planning to survey
approaches and document tradeoffs. This creates a `research` entity (RES-xxxx)
with structured sections: Problem, Context, Options, Recommendation.

**Workflow:**

1. `/research` creates the entity in `in-progress` and links it to concepts
2. The agent surveys the codebase and external approaches
3. Options are documented with pros/cons/effort
4. A recommendation is made and presented for user review
5. The research is linked to the ticket/feature via `has-research`

**When to use:** Enhancements or features where the approach isn't obvious,
multiple viable options exist, or the change touches unfamiliar subsystems.
The planning checklist has a research item that can be skipped with N/A for
smaller work.

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

Status transitions auto-create checklists (and similar side effects) via
automations declared in the project's `metamodel.yaml`. Action types
(`set`, `create_relation`, `create_entity` with `if_exists`) and
interpolation patterns (`{{new.property}}`, `{{entity.id}}`, `{{today}}`)
are documented in `docs/metamodel.md` and exemplified in the live
`metamodel.yaml`. Read those rather than relying on a copy here — a stale
copy is worse than a pointer.

Common mistake: `{{entity.title}}` is wrong; use `{{new.title}}` for a
property of the triggering entity.
<!-- @managed: claude-workflow end -->
