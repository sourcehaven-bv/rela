# CLAUDE.md

## Rules for new code

- **Define interfaces at the call site, not next to the implementation.**
  Producer-side interfaces couple consumers to every method the producer
  exposes. Each consumer declares the minimum interface it needs —
  usually one to three methods.
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
- **Don't extend `internal/workspace` in new code.** It is a transitional
  shim still wired in `cli/root.go` and `mcp/server.go`. New call sites
  take the focused interfaces above directly. When touching code that
  still uses workspace, prefer migrating it out over extending.

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

### Validation policy for write APIs

rela's storage format is permissive: markdown + YAML frontmatter, edited
freely by external tools alongside the API. The philosophy is **tolerate
temporarily invalid data**; the `analyze_*` tools surface inconsistencies
that the storage layer doesn't reject.

Write-time checks split into three classes (DEC-HWZHA):

| Class | When | HTTP |
|---|---|---|
| **Hard 400 — malformed wire format** | Request structure is broken, detectable without the metamodel | 400 |
| **Hard 422 — structural impossibilities** | Storage layer literally cannot persist this | 422 |
| **Write-with-warnings (200 + warnings)** | Soft conditions: target type mismatch, missing target, unknown meta keys, required-meta unset, meta type mismatches | 200 |

The 200-with-warnings path performs the requested write and returns
warnings in the response body so UIs surface them non-blockingly. Each
warning is `{code, path, detail}` where `code` matches the corresponding
`analyze_*` finding code so UIs can de-duplicate against analyze runs.

Resist drift toward hard rejection on soft conditions. JSON:API and
similar wire formats bring a "validate-then-422" mental model from
REST-over-database stacks where wire and storage share a closed schema;
rela's storage is intentionally more permissive than that. If you find
yourself adding a 422 on a write path, ask: "could a hand-editor produce
this state in a markdown file? If yes, it's a soft condition — warn,
don't reject."

### Packages

Entry points: `cmd/rela`, `cmd/rela-server`, `cmd/rela-desktop`.

Domain and storage:

| Package                  | Purpose                                                   |
| ------------------------ | --------------------------------------------------------- |
| `internal/entity`        | Domain types: `Entity`, `Relation` (no storage metadata)  |
| `internal/metamodel`     | Schema: entity types, relations, properties, validation   |
| `internal/store`         | Storage abstraction — CRUD + events, `fsstore`/`memstore` |
| `internal/tracer`        | Pure-reader graph traversal (trace, path, orphans, cycles)|
| `internal/search`        | Full-text + structured search (bleve + linear)            |
| `internal/entitymanager` | Write path: automations, validation, policy               |
| `internal/validator`     | Validation engine invoked by entitymanager                |
| `internal/markdown`      | Parse/write entity and relation markdown                  |
| `internal/project`       | Project discovery, paths (`Context`)                      |
| `internal/workspace`     | Legacy aggregate — transitional, being phased out         |

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
| `internal/ai`         | OpenAI-compatible LLM provider (used from Lua)                 |
| `internal/migration`  | Schema migrations for project YAML files                       |

Other packages under `internal/` are self-descriptive — ls the tree.

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

`govulncheck` runs on every PR that touches `go.mod` / `go.sum` (blocking,
`vulncheck` job in `ci.yml`) and weekly from `security.yml`. The weekly run
auto-updates called-and-fixable vulns via `go get` + `go mod tidy` and opens
an auto-merge PR on success, or files / updates a deduplicated issue on
failure.

Known-unfixable vulnerabilities are filtered via
`scripts/govulncheck-filtered.sh`; keep `IGNORED_OSVS` in sync between that
script and `scripts/govulncheck-fixable.sh`. Run locally: `just govulncheck`.

## Commands

Read the `justfile` for the full set. The shortcuts used most often:

- `just build` — build all binaries
- `just test` — race-enabled test run
- `just lint` / `just lint-fix` — golangci-lint
- `just arch-lint` — package boundary check
- `just ci` — run the full CI pipeline
- `just dev` — run the data entry server locally
- `go test -run TestName ./...` — single test

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
