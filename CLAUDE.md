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

### Consumer-side interfaces for callbacks and cycles

The "interfaces at the call site" rule has a specific application that is
worth calling out: **when service A needs to call back into something that
also holds A, do not reach for the concrete type or a shared interface
package — define a small interface in A's package describing the methods
A actually invokes, and have the wiring site supply an implementation.**

This is how rela avoids constructor cycles, how it keeps interface
surfaces small, and how it prevents new collaborators from leaking into
unrelated test setups.

**Why it matters.** If service A imports type B because A needs to call
B's methods, and B imports A as a dependency, you have a constructor
cycle. The reflexive fixes — late-binding setters, a shared interface
package, passing pointers around — all add indirection without solving
the underlying design problem. A consumer-side interface dissolves the
cycle by letting A declare *exactly the contract A needs* without A
having to know which concrete type satisfies it.

**Three layers, in order:**

1. **A defines a small interface (`Host`, `Mutator`, `Provider`, etc.)**
   in its own package. The interface names only the methods A invokes —
   typically two to four. No options structs A doesn't pass; no methods
   A never calls.

2. **A's constructor and methods accept that interface.** Either as a
   constructor field (if A holds the relationship for its lifetime) or
   as a per-call argument (if the relationship is per-invocation).
   Per-call is cleaner when the implementer is the *caller* of A's
   method — the cycle disappears entirely because A has no permanent
   reference.

3. **The wiring site supplies the implementation.** This is usually the
   concrete type that *also* depends on A. The wiring is straightforward
   because both types exist by the time the wiring code runs.

**Worked example — what the pattern looks like.** The
`autocascade.Runner` service (in `internal/autocascade`) needs entity-
and relation-creating callbacks during automation cascades. Today
`Workspace` satisfies its `Host` interface; once `entitymanager.Manager`
exists, that will satisfy it instead. Both can hold the Runner without
a constructor cycle because Host is supplied per-call.

```go
// internal/autocascade/host.go
package autocascade

// Host is what Runner needs from its caller. Defined here, at the
// consumer, naming only the methods Runner invokes — not the full
// EntityManager / Workspace surface.
type Host interface {
    Meta() *metamodel.Metamodel
    Store() store.Store
    CreateEntityNoCascade(entityType string, opts CreateEntityOptions) (*entity.Entity, error)
    WriteEntity(e *entity.Entity) error
    WriteRelation(r *entity.Relation) error
    DeleteEntity(ctx context.Context, entityType, id string, cascade bool) error
    FindExistingRelationTarget(sourceID, relationType, targetType string) *entity.Entity
}

type Runner struct {
    engine  *automation.Engine
    scripts script.Executor
    // No Host field — Host is per-call.
}

// Process accepts Host on the call, not at construction.
func (r *Runner) Process(ctx context.Context, host Host, req Request) (Outcome, error) {
    // ... uses host.CreateEntityNoCascade, host.WriteRelation, etc. ...
}
```

```go
// internal/entitymanager/manager.go (future — TKT-QTNX)
package entitymanager

type Manager struct {
    store  store.Store
    runner *autocascade.Runner // Manager holds Runner.
    // ... no late-binding back-reference; no setter; no cycle ...
}

func (m *Manager) CreateEntity(ctx context.Context, e *entity.Entity) (*Result, error) {
    // ... validate, write to store ...
    outcome, err := m.runner.Process(ctx, m, autocascade.Request{Trigger: e, ...})
    //                                  ^ m satisfies autocascade.Host implicitly
    // ... merge outcome, return ...
}
```

`Manager` satisfies `autocascade.Host` *structurally* — there is no
import of `autocascade.Host` in `entitymanager`, no declaration of
"implements," nothing. The cycle disappears because `Runner` doesn't
hold a reference; it borrows one for the duration of `Process`.

**When to use which form:**

- **Per-call argument** when the implementer is the *caller* of the
  consumer's method (as in the example above). This is the form that
  fully dissolves cycles.
- **Constructor field** when the consumer holds the relationship across
  many calls (e.g., `mcp.Services` in `internal/mcp/server.go`,
  `scheduler.WorkspaceProvider` in `internal/scheduler/scheduler.go`).
  Used when there is no cycle, just a desire to keep the consumer's
  contract narrow.

**Existing examples in the codebase to study:**

- **`mcp.Services`** at `internal/mcp/server.go` — scoped consumer-side
  interface; Workspace satisfies it. Constructor-field form. Keeps MCP
  independent of Workspace's full surface.
- **`scheduler.WorkspaceProvider`** at `internal/scheduler/scheduler.go`
  — four-method interface for what the scheduler needs from a
  workspace. Constructor-field form.
- **`store.EntityObserver`** at `internal/store/store.go` — the
  *inverse* shape: store calls *out* to its observers, and observers
  declare what they implement. Search index uses it. Per-call form
  (observer is invoked per event).

**What this pattern rules out:**

- **Concrete type back-references** (`A holds *B; B holds *A`).
- **Late-binding setters** (`a.SetB(b)` after construction). They turn
  type errors into runtime nil-deref bugs.
- **Shared "interfaces" package** that exists only to break cycles.
  Dissolving cycles with a single shared package leaks every
  participating type into every consumer's test imports.
- **Producer-side interfaces** that publish every method a service
  exposes, on the theory that "consumers can pick what to use." They
  can't — Go doesn't unify partial implementations of a wide interface
  into a narrow one. The narrow interface has to live with the
  consumer.

**Test consequence.** A test for the consumer becomes a stub of the
narrow interface — three methods to mock, not the full producer
surface. Tests that ran with a real `Workspace` fixture in 30 lines of
setup can run with a 5-line stub.

#### Narrow on returns, not just methods

If a consumer-side interface returns a broad type only so the caller
can invoke one or two methods on it, declare those methods on the
interface directly. Returning the wide type is a soft leak — it tells
the implementer "I need everything this type can do" when in fact you
only need a slice.

```go
// Wrong: leaks the whole metamodel + store surface for two narrow uses.
type Host interface {
    Meta() *metamodel.Metamodel  // only used for meta.ValidateRelation(...)
    Store() store.Store          // only used for store.GetEntity(ctx, id)
}

// Right: declares the actual operations.
type Host interface {
    ValidateRelation(relType, fromType, toType string) error
    GetEntity(ctx context.Context, id string) (*entity.Entity, error)
}
```

The narrow form's payoff is concrete: `autocascade.Host` collapsed
its arch-lint footprint from `[automation, metamodel, store]` to
`[automation]` once `Meta()` and `Store()` were replaced with the
methods Runner actually invoked.

The verification question is "where does the consumer call this?"
If the answer is "in exactly one place to call exactly one method,"
that's the method that should be on the interface.

#### Names declare contracts; docs declare invariants

Method names should describe *what's requested*. Behavioral
constraints belong in the doc comment, not in the name.

```go
// Wrong: name encodes a "must not" into the contract surface.
CreateEntityNoCascade(...) (*entity.Entity, error)

// Right: name describes the operation; doc carries the constraint.
//
// CreateEntity creates a new entity from the supplied options.
//
// Contract: the implementation must NOT fire follow-up automation
// cascades from within this call. Runner is the one that schedules
// cascade evaluation on the returned entity; double-cascading would
// enforce MaxDepth twice.
CreateEntity(...) (*entity.Entity, error)
```

Behavioral negatives in names ("NoX", "WithoutY", "NonZ") are usually
the author prescribing implementation strategy. The doc form is more
honest — implementers are free to satisfy the constraint however they
want.

#### Transport-specific types belong at adapter layers

If a consumer's interface references types from a specific runtime
(`lua.WriteDeps`, `http.Request`, `*sql.Rows`), the consumer has
absorbed knowledge it doesn't need. The package now imports the
runtime's package, and every alternative implementation must speak
that runtime's vocabulary.

The fix: define an abstract interface in the consumer; build a
per-request adapter at the wiring site that holds the transport-
specific state.

```go
// Wrong: consumer's interface is shaped by Lua's API.
type Executor interface {
    ExecuteCode(code string, deps lua.WriteDeps, e, old *entity.Entity) error
    ExecuteFile(path string, deps lua.WriteDeps, e, old *entity.Entity) error
}

// Right: consumer declares only what it needs to ask for; transport
// lives in the adapter at the wiring site.
type ScriptRunner interface {
    Run(ctx context.Context, action ScriptAction) error
}

// In workspace (the wiring site), per-request:
type luaScriptRunner struct {
    exec script.Executor
    deps lua.WriteDeps  // bound at this layer
}
func (l *luaScriptRunner) Run(ctx context.Context, a ScriptAction) error {
    // engine-specific dispatch + error formatting lives here
}
```

The cost is one adapter file per transport. The win is that the
consumer's package doesn't import the transport's package and a
second transport can plug in without touching the consumer. Concrete
example: `autocascade` stopped importing `internal/lua` once the
`ScriptRunner` adapter pattern landed; the cycle that would have
blocked `entitymanager.Manager → autocascade → lua → entitymanager`
dissolved at the same time.

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
- **Don't run user-supplied Lua on the read path.** ACL gates and
  filters evaluate against declarative policy (`acl.yaml`) and the
  graph; Lua participates only at *write time* via the automation
  engine (which produces graph relations the ACL reads). The
  cross-system survey behind `internal/acl/` (see
  `.ignored/acl-design.md`) shows that per-row Lua on reads is the
  perf cliff every comparable system regrets; the project avoids it
  by design.

### Authorization (`internal/acl`)

The ACL is consumed by `entitymanager.Manager` (a required
collaborator via `Deps.ACL`) and surfaces structured 403s in
`internal/dataentry`. Three production implementations live in
`internal/acl`:

- `NopACL` — allow-all; default when no `acl.yaml` is present.
- `ReadOnlyACL` — deny-all; wired via `rela-server --read-only`.
- `Declarative` — policy-driven, composed with a `Policy` loaded
  from `acl.yaml` at the project root.

Consumer-side interface rule: code that calls into the ACL declares
the narrowest contract it needs at the call site, not `acl.ACL`
in full, when only a subset of methods are invoked. `entitymanager`
is the exception — it owns the constructor field so the wiring
boundary is explicit.

See `docs/security.md` for the user-facing schema reference,
`.ignored/acl-design.md` for the design rationale and the four-layer
model (users → groups → roles → local roles), and `docs/audit-log.md`
for the `denied-write` audit op.

### Action affordances (`_actions`)

The data-entry API attaches a per-resource `_actions: map[string]bool`
to every entity and list response. The SPA reads it to decide which
write controls to render. The map is a **UI hint** — the server
re-authorizes every write.

Rules for new write code in `internal/dataentry`:

- **Route every `acl.WriteRequest{Op:...}` through `translateVerb`**
  in `internal/dataentry/affordances.go`. A grep test
  (`lint_test.go`) enforces this: no other file in `internal/dataentry`
  may construct the literal. The shared constructor is the structural
  guarantee that the affordance map and the actual write resolve to
  the same ACL request.
- **Don't trust `_actions` for authorization decisions.** The write
  endpoint must re-authorize. The bidirectional contract test in
  `affordances_contract_test.go` pins the invariant: every
  `_actions[v] == false` ⇒ 403 on the corresponding write, every
  `true` ⇒ 2xx.
- **New verbs require coordinated changes:** add an `acl.Op`
  constant, add a `translateVerb` case, and update
  `docs/data-entry/api-reference.md`. Old SPAs ignore unknown keys;
  removing or renaming a verb is a major API version bump.
- **Phase 1 verbs:** `create` (per-collection), `update` / `delete` /
  `rename` (per-item). `transition:*` and `relation:*` are deferred
  until ACL gains Op variants or extension fields.

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

### Audit log

Every successful entity / relation create / update / delete /
rename is audited by `entitymanager.Manager` as a JSONL record
under `.rela/audit/YYYY-MM-DD.jsonl`. See `docs/audit-log.md` for
the user-facing reference; rules for new code:

- **New write paths inherit audit automatically.** Any code that
  calls `entitymanager.Manager.{Create,Update,Delete,Rename}{Entity,Relation}`
  produces a record without further wiring. Do not bypass Manager
  by writing directly to `store.Store` from a write path — the
  audit record won't be emitted.
- **New entry-point binaries stamp Principal at startup.** Each
  binary or root command attaches a Principal once:
  `ctx = principal.With(ctx, principal.Principal{User: principal.SystemUser(), Tool: principal.ToolXxx})`.
  Use one of the `principal.ToolCLI` / `ToolMCP` / `ToolDataEntry` /
  `ToolScheduler` / `ToolDesktop` constants — string literals will
  not surface typos until the entry-point smoke test catches them.
- **Engine-initiated paths stamp `triggered_by`.** Scheduler tasks
  wrap the per-task ctx with `audit.WithTriggeredBy(ctx, "schedule:"+task.Name)`;
  the autocascade runner does the analogous thing for automation
  cascades. Direct user actions leave `triggered_by` empty.
- **Lua bindings do not expose audit primitives.** A Lua script
  must not be able to call `principal.With` or rewrite its
  own attribution — the spoofing test in `internal/lua/audit_spoofing_test.go`
  guards this. Do not register `rela.audit` or `rela.principal`
  on the runtime.
- **Constructor takes `Audit` as a required collaborator.**
  `entitymanager.Deps.Audit` and `appbuild.New` reject nil.
  Tests use `audit.Nop{}` (an explicit opt-out) or `audit.NewMemory()`
  when they assert on records.

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
