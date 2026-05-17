---
id: PLAN-XKMJ
type: planning-checklist
title: 'Planning: Audit log: append-only JSONL of entity write operations'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Refactor note (2026-05-17).** This plan originally targeted
> `internal/workspace.wsEntityManager`. That package was deleted during the
> workspace-decomposition arc (TKT-QTNX → IU2S → DS43 → UG3C → 64R3 / 2IAC).
> Approach has been rewritten against the current world: write chokepoint is
> `entitymanager.Manager`, wiring facade is `appbuild.Services`. Acceptance
> criteria and design decisions are unchanged in substance; only the file
> paths and constructor names move.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In:**

- New `internal/audit` package with a small `Audit` interface and three backends:
  - **`Nop`** — no-op, used by tests and explicit opt-out paths.
  - **`Filesystem`** — JSONL writer under `.rela/audit/YYYY-MM-DD.jsonl`, daily UTC rotation, append-only, internal mutex.
  - **`Memory`** — in-memory slice backend used in integration tests so assertions can read records back without filesystem fixtures.
- `Audit` becomes a **required collaborator on `entitymanager.Deps`** (validated by `entitymanager.New`). Production wiring through `appbuild.Discover`; test sites pass `audit.NewMemory()` or `audit.Nop{}` via a new `WithTestAudit(...)` option on `appbuild.NewForTest`.
- `Manager` (`internal/entitymanager/manager.go`) calls `Audit.Record` on every successful create/update/delete/rename — for both entities and relations. Recording happens once per op, after the store write returns.
- **Structured `Principal` type lands in this PR**, not later. `Principal{User, Tool}` where `User` is the OS user (`$USER` with fallback to `"unknown"`) and `Tool` is one of `"cli" | "mcp" | "data-entry" | "scheduler" | "desktop"`. The wiring site sets `Tool` once at process start; `User` is captured at the same time. `data-entry` will later override `User` per-request from a header (out of scope for this PR; the type is ready for it).
- `Principal` is carried via `context.Context` — both at the entry point (so the wiring site can stamp it once) and on every audit record. `triggered_by` continues to carry the engine-attribution string (`automation:<name>`, `schedule:<task-name>`), orthogonal to `Principal`.
- All Manager methods already take `context.Context`. The audit hook reads both `audit.PrincipalFrom(ctx)` and `audit.TriggeredByFrom(ctx)`.
- Constructors `audit.NewFilesystem(...)` / `audit.NewMemory(...)` reject empty/invalid required fields per project's "constructors reject nil required fields" rule.

**Out (subsequent phases):**

- Per-request principal override in data-entry (the type accommodates it; the HTTP middleware that reads a header and overrides `Principal.User` is a follow-up).
- Write-policy hook (the Manager dispatch this ticket establishes will be reused by it).
- `outcome: denied` records — no policy yet means no denials. Schema admits optional `outcome` later.
- UI rendering, CLI helpers (`rela audit tail`), search/query over audit data.
- Read-side audit (no logging of reads).
- Log retention/cleanup policy. Documented as an operator concern.

**Decisions (confirmed with user, unchanged from original):**

1. **No `outcome` field in v1.** Nothing to record outcome of yet — every write that reaches the audit hook is by definition successful. When write-policy lands, the schema gains an optional `outcome: "denied"` field; absence continues to mean success.
2. **Audit failure does not block the write.** `slog.Error` and continue. No retry, no buffering, no propagation up the write path. Audit is forensic; an unwritable `.rela/audit/` directory (disk full, perms) shouldn't refuse legitimate edits. Operators monitor for `audit.write_failed` log events.
3. **Automation cascades attribute to the innermost automation.** Matches what `autocascade.Runner` already carries via `LuaToExecute.AutomationName` (or its successor field in the post-decomposition Runner).

**Acceptance Criteria:**

1. **AC1: every entity write produces one audit record.** Create, update, delete, rename — for each, exactly one record reaches the configured backend. Table-driven test in `internal/entitymanager/` using the `Memory` backend.
2. **AC2: every relation write produces one audit record.** Same shape for create/update/delete relation.
3. **AC3: Principal flows from ctx into the record.** Set `audit.WithPrincipal(ctx, Principal{User: "alice", Tool: "cli"})` → resulting record carries `principal.user="alice"` and `principal.tool="cli"`. Absence of a principal in ctx → record carries `principal.user="unknown"`, `principal.tool="unknown"`.
4. **AC4: each entry point stamps the right Tool.** Smoke tests at the wiring layer (one per entry point: cli root, mcp server, data-entry server, scheduler, desktop) confirm `Principal.Tool` is the expected literal.
5. **AC5: triggered_by populated for automation-driven writes.** Integration test in `internal/entitymanager/`: metamodel with an `on: created` automation that creates a child entity. Triggering write produces two records — the user write with empty `triggered_by`, the cascade with `triggered_by: "automation:<name>"`. Both records carry the same Principal.
6. **AC6: triggered_by populated for scheduler-driven writes.** Integration test in `internal/scheduler/`: scheduler runs a Lua task that calls `rela.create_entity`; resulting record carries `triggered_by: "schedule:<task-name>"` and `principal.tool="scheduler"`.
7. **AC7: daily rotation.** With an injected clock, first record lands in `2026-05-10.jsonl`, second (across the boundary) in `2026-05-11.jsonl`.
8. **AC8: append-only correctness under concurrency.** N parallel writes through Manager produce N intact JSONL lines. (Manager has no shared writer mutex of its own; `Filesystem` carries an internal mutex. Tested explicitly with `-race`.)
9. **AC9: audit failure does not block the write.** A backend whose `Record` triggers an internal error still allows the entity write to succeed; a `slog.Error` is emitted (`audit.write_failed`). No retry, no buffering. *Note: `Audit.Record` is a no-return-value method — backends self-log; Manager never sees a returned error.*
10. **AC10: Nop is safe.** `entitymanager.Deps{... Audit: audit.Nop{}}` works without panic and without writing anything.
11. **AC11: nil Audit is rejected at construction.** `entitymanager.New` validates `Deps.Audit != nil` and returns an error; `appbuild.Discover` validates similarly. (`appbuild.NewForTest` carve-out: substitutes `audit.Nop{}` when no `WithTestAudit` is supplied, so tests don't have to spell it out.)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Logging libraries with pluggable sinks — surveyed, rejected.** A reviewer asked whether a Go logging lib with flexible sinks could replace the hand-rolled writer. Concrete options considered:
  - **`slog.JSONHandler` + `lumberjack`** (the popular pairing). lumberjack rotates by **size**, not date. To get `YYYY-MM-DD.jsonl` filenames you'd need a midnight-cron goroutine that calls `Rotate()` and post-renames the output — already half a hand-roll. Lumberjack also hardcodes file mode 0o644 and dir mode 0o744; we want 0o600 / 0o700.
  - **`slog.JSONHandler` + `rotoslog`**. Time-based rotation, but filenames are `YYYY-MM-DD-<suffix>`, not our pattern. Project is small / low-traffic.
  - **slog itself** forces `time`, `level`, `msg` keys onto every record. Our record shape is `{time, op, entity_type, entity_id, principal, triggered_by, summary}` — no `level`, no `msg`. We'd need a `ReplaceAttr` callback to strip `level` and rewire `msg` onto `summary`. Workable, but we're fighting the handler to remove its idioms rather than gaining flexibility from it.
  - **Existing audit-specific libraries** (`audit-go`, OPA decision logs) are RBAC/policy-shaped — adopting one would force premature schema decisions about subject/object/action and a policy decision-point we don't have.
- **Decision: stdlib hand-roll.** `encoding/json` + `os.OpenFile(O_APPEND|O_CREATE|O_WRONLY, 0o600)` + an internal mutex + a midnight check on each write. Total: ~80 lines. Zero dependencies. Exact filename, mode, and rotation control. The library options would all save fewer lines than they add in workarounds.
- **slog stays the project's operational-logging standard** (`.golangci.yml` enforces `log/slog`). Audit JSONL records align field-naming conventions with slog (`time` not `timestamp`, lower-snake-case keys) for visual consistency, but write through a dedicated writer rather than via slog because audit-stream needs append-on-disk with a fixed file layout, and mixing audit with operational logs would make `tail -f` either too noisy or require fragile filtering.
- **`.rela/scheduler-state.json`** (`internal/scheduler/scheduler.go`) is the closest existing "engine writes JSON under .rela/" pattern — same `filepath.Join(paths.CacheDir, ...)` construction. Audit follows that.

**Reference patterns in codebase (post-decomposition):**

- `internal/entitymanager/manager.go` — `Manager` is the sole write chokepoint. Each write method already takes `context.Context`. `Deps` is explicitly designed to accept new required collaborators (see the doc comment at line 72-74: *"Using a struct keeps the constructor signature stable as new collaborators land (audit, principal, policy in subsequent tickets)."*).
- `internal/appbuild/appbuild.go` — `Services` constructs `Manager` once per process. Adding `Audit` here propagates to CLI / server / desktop / scheduler in one stroke.
- `internal/appbuild/testfixture.go` — `NewForTest` is the single test fixture. The `TestOption` pattern (e.g. `WithTestStore`, `WithFS`) extends naturally: add `WithTestAudit`.
- `internal/autocascade/runner.go` — already carries the automation name through cascade execution. The cascade-internal mutator that re-enters Manager methods is where `audit.WithTriggeredBy(ctx, "automation:"+name)` gets set.
- `internal/scheduler/scheduler.go` — single entry point invokes the script engine per task. Wrap that call's `context.Context` with the schedule name there.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Package `internal/audit` — types:**

   ```go
   // Principal identifies who is making a request. User is the OS user
   // captured at process start (will be overridable per-request in
   // data-entry, via an HTTP middleware, in a follow-up PR). Tool
   // identifies the entry point: "cli" | "mcp" | "data-entry" |
   // "scheduler" | "desktop".
   type Principal struct {
       User string `json:"user"`
       Tool string `json:"tool"`
   }

   type Record struct {
       Time        time.Time `json:"time"`
       Op          string    `json:"op"`
       EntityType  string    `json:"entity_type"`
       EntityID    string    `json:"entity_id"`
       Principal   Principal `json:"principal"`
       TriggeredBy string    `json:"triggered_by,omitempty"`
       Summary     string    `json:"summary,omitempty"`
   }

   type Audit interface { Record(rec Record) }

   type Nop struct{}
   type Memory struct{ /* mutex + records */ }
   type Filesystem struct{ /* dir, mutex, current file/date */ }

   func NewFilesystem(dir string) (*Filesystem, error)
   func NewMemory() *Memory

   // SystemUser resolves the OS user at process start: $USER → "unknown".
   // No git fallback, no $RELA_ACTOR escape hatch — the previous chain
   // was over-engineered for what's effectively one bit of operator
   // identity. Operators who need a different identity set $USER (or,
   // in a future PR, send a header to data-entry).
   func SystemUser() string
   ```

The `Audit` interface is single-method and consumer-side, per CLAUDE.md.
`Memory.Records()` returns a snapshot for test assertions. The `Filesystem`
backend no longer captures actor at construction — it reads `Principal` from
each `Record` instead.

2. **Principal + triggered_by plumbing via `context.Context`** — every
   `Manager` method already takes `ctx`; we start *using* it for two values.

   ```go
   // internal/audit/context.go
   type principalKey struct{}
   type triggeredByKey struct{}

   func WithPrincipal(ctx context.Context, p Principal) context.Context {
       return context.WithValue(ctx, principalKey{}, p)
   }

   func PrincipalFrom(ctx context.Context) Principal {
       if v, ok := ctx.Value(principalKey{}).(Principal); ok { return v }
       return Principal{User: "unknown", Tool: "unknown"}
   }

   func WithTriggeredBy(ctx context.Context, label string) context.Context { /* ... */ }
   func TriggeredByFrom(ctx context.Context) string                       { /* ... */ }
   ```

   Each entry point stamps `ctx = audit.WithPrincipal(ctx, audit.Principal{
   User: audit.SystemUser(), Tool: "<this-entry-point>"})` once at startup
   (or per-request for data-entry, eventually). Manager reads
   `audit.PrincipalFrom(ctx)` and `audit.TriggeredByFrom(ctx)` on each
   write. No new field on `Deps` beyond `Audit` itself. No goroutine-local
   state. No mutex dance.

3. **Manager hooks.** Add a private helper:

   ```go
   func (m *Manager) recordAudit(ctx context.Context, op, entityType, entityID, summary string)
   ```

   Invoke on each of the 7 write methods' tail-success branch (CreateEntity,
   UpdateEntity, DeleteEntity, RenameEntity, CreateRelation, UpdateRelation,
   DeleteRelation). The helper reads `audit.PrincipalFrom(ctx)` and
   `audit.TriggeredByFrom(ctx)`.

4. **Autocascade plumbing.** `autocascade.Runner` (or its per-cascade mutator)
   wraps the ctx with `audit.WithTriggeredBy(ctx, "automation:"+name)` before
   re-entering `Mutator` methods. Principal is *not* overwritten — the cascade
   inherits the originator's Principal, which is the desired attribution.

5. **Scheduler plumbing.** `internal/scheduler/scheduler.go` derives:

   ```go
   ctx := audit.WithPrincipal(parent, audit.Principal{
       User: audit.SystemUser(), Tool: "scheduler",
   })
   ctx = audit.WithTriggeredBy(ctx, "schedule:"+task.Name)
   ```

   immediately before invoking the script engine. Lua bindings call Manager
   through the same ctx; both values flow naturally.

6. **Per-entry-point Principal stamping.** Each entry point binary or root
   command attaches `Principal` once:
   - `cmd/rela` / `internal/cli/root.go`: `Tool: "cli"`.
   - `cmd/rela-server` (the data-entry server): `Tool: "data-entry"`. Per-request override (from a header) is out of scope for this PR; the entry point sets a process-wide default.
   - `cmd/rela-desktop`: `Tool: "desktop"`.
   - `internal/mcp` server: `Tool: "mcp"`.
   - `internal/scheduler`: `Tool: "scheduler"` (set in step 5).

   The `User` is `audit.SystemUser()` everywhere (which is `$USER` with
   fallback to `"unknown"`).

7. **Production wiring.**
   - `appbuild.Discover` constructs `audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))` and passes it into `entitymanager.New` via `Deps.Audit`.
   - `appbuild.New` (the lower-level constructor used by tests-of-Discover and any future custom wiring) requires `Audit` and rejects nil.
   - `appbuild.NewForTest` adds `WithTestAudit(audit.Audit) TestOption`; if not supplied, defaults to `audit.Nop{}`.
   - Principal stamping is *not* `appbuild`'s job — it's an entry-point concern (different binaries / root commands use different Tool values). `appbuild` returns a `Services` bundle; the caller wraps its ctx with `WithPrincipal` at the call site.

**Files to modify (current paths):**

- **New:** `internal/audit/audit.go` (interface, `Record`, `Principal`), `internal/audit/nop.go`, `internal/audit/memory.go`, `internal/audit/filesystem.go`, `internal/audit/context.go`, `internal/audit/user.go` (`SystemUser`), plus `_test.go` for each.
- `internal/entitymanager/manager.go` — add `Audit audit.Audit` to `Deps`; `New` rejects nil; add `recordAudit` helper; invoke from the 7 write methods.
- `internal/entitymanager/manager_test.go` and integration tests — assert audit calls via `audit.NewMemory()`; use `audit.WithPrincipal` in test fixtures.
- `internal/appbuild/appbuild.go` — `Discover` constructs `audit.NewFilesystem` and threads through; `New` validates required Audit.
- `internal/appbuild/testfixture.go` — add `WithTestAudit(audit.Audit) TestOption`; default to `audit.Nop{}` in `mustBuildTest...` helpers.
- `internal/cli/root.go` — wrap root cobra context with `audit.WithPrincipal(ctx, Principal{User: audit.SystemUser(), Tool: "cli"})`.
- `cmd/rela-server/main.go` — same with `Tool: "data-entry"`.
- `cmd/rela-desktop/main.go` — same with `Tool: "desktop"`.
- `internal/mcp/server.go` — same with `Tool: "mcp"` at request-handler entry.
- `internal/autocascade/runner.go` (or its mutator-adapter file) — wrap ctx with `audit.WithTriggeredBy(...)` before Mutator calls. *Verify the actual file during implementation; the runner internals shifted with the cascade extraction (TKT-6OMC).*
- `internal/scheduler/scheduler.go` — wrap script-engine ctx with both `WithPrincipal(... Tool: "scheduler")` and `WithTriggeredBy(ctx, "schedule:"+task.Name)`.
- Test files: any callers that construct Manager directly (rare — most go through appbuild) get `Audit: audit.Nop{}` or `audit.NewMemory()`.
- A docs page covering `.rela/audit/` location, record shape, and operator concerns (manual rotation/retention).

**Note on `lua.WriteDeps`:** Originally the plan added `Audit` to
`lua.WriteDeps`. That's no longer necessary. Lua bindings call
`WriteDeps.EntityManager` (the `Mutator` interface), and the concrete
`*entitymanager.Manager` behind it already holds the Audit. Lua-driven writes
are audited automatically via Manager's recordAudit; no separate plumbing on
`WriteDeps` is required. This also keeps `lua.WriteDeps` narrow and
consumer-side (per the recent refactor that made `Mutator` a 5-method
interface).

**Alternatives considered (rejected):**

- **Bolt audit into the store layer.** Multiple stores (memstore, fsstore) each get instrumented and tests for each backend drag in audit concerns. Manager is already the chokepoint for "human intent" — the right layer.
- **slog.JSONHandler + lumberjack/rotoslog** (researched in response to round-2 review feedback). Lumberjack rotates by size, not date — getting `YYYY-MM-DD.jsonl` filenames requires a midnight-cron goroutine, file modes are hardcoded (0o644 / 0o744 vs our 0o600 / 0o700), and slog itself bakes `level` and `msg` keys into every record, fighting our schema. Hand-rolling the JSONL writer is ~80 lines with zero deps and exact filename / mode control.
- **`audit-go` / OPA decision logs.** RBAC/policy-shaped — would force premature schema decisions about subject/object/action and a policy decision point we don't have.
- **Async audit (channel + worker).** Synchronous writes are simple, correctness-obvious, and adequate given write throughput is low and already serialized. Async can be added later behind the same interface.
- **Add `Audit` to `lua.WriteDeps`.** Original plan did this. No longer needed — Lua bindings call `WriteDeps.EntityManager` which is `*entitymanager.Manager`, and that already holds Audit.
- **Goroutine-local / mutex-swap state** for Principal / triggered-by. Rejected — `context.Context` is Go's idiom and is already plumbed end-to-end through Manager.
- **Defer Principal to a follow-up PR** (the original plan). Rejected in round 2 — the whole point of this ticket is establishing the operation-context plumbing that later phases extend. A `string actor` is a future migration; a `Principal{User, Tool}` struct is forward-compatible from day one (add `principal.email`, `principal.session_id`, etc., without breaking readers).
- **Elaborate actor fallback chain** (`$RELA_ACTOR` → `$USER` → `git config user.email` → `"system"`). Rejected in round 2 — over-engineered. `$USER` with `"unknown"` fallback is enough; operators who want a different identity set `$USER`, or (in a follow-up) data-entry middleware reads a request header.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`$USER` env var** (via `audit.SystemUser()`) — trimmed; capped at 256 chars; printable UTF-8 only (control chars stripped). Empty or invalid → `"unknown"`. No fancier chain (originally proposed `$RELA_ACTOR` + git fallback; dropped as over-engineered for what's one bit of operator identity).
- **`Principal.Tool`** — set by the wiring site from a fixed allowlist (`"cli" | "mcp" | "data-entry" | "scheduler" | "desktop"`). Not user-controlled. If a future caller passes an unknown value, it goes through unchanged — type-system enforcement isn't worth a sealed-type pattern in Go, but `audit_test.go` includes a smoke test that enumerates the expected values per entry point.
- **Entity IDs / types** — already validated upstream; we stringify and let JSON encoding handle escaping.
- **`triggered_by` value** — sourced from metamodel-loaded automation/schedule names (validated at load). Length-capped + control-char stripped at write time as defense-in-depth against a metamodel author putting a newline in a name and corrupting the JSONL stream.

**Security-Sensitive Operations:**

- **File creation under `.rela/audit/`** — `os.MkdirAll(dir, 0o700)`, files opened with `0o600`. Path is `filepath.Join(cacheDir, "audit", date+".jsonl")` where `date` is `time.Now().UTC().Format("2006-01-02")` — no user input enters the path.
- **Audit records can contain entity IDs and types** — these are not secrets in rela's model (entities are markdown files in the project tree).
- **No property values logged** in v1's `summary`. Only "created", "updated status: ready→done", "deleted". Property *names* may appear; values do not — defense against secrets accidentally stored in properties.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria above — each AC maps to a named
test.

**Unit tests** (`internal/audit/`):

- `Record` + `Principal` JSON round-trip: marshal/unmarshal, omitempty behavior on `triggered_by` and `summary`.
- `NewFilesystem` rejects empty dir.
- `NewMemory` returns a working backend; `Records()` is a snapshot (mutating it doesn't affect the backend).
- Daily rotation across an injected clock boundary.
- Concurrent `Record` calls produce N intact JSONL lines (no interleaving).
- `SystemUser` returns `$USER` when set, `"unknown"` when not.
- `WithPrincipal` / `PrincipalFrom` round-trip; absence returns `Principal{User: "unknown", Tool: "unknown"}`.
- `WithTriggeredBy` / `TriggeredByFrom` round-trip; absence returns empty string.
- Filename construction for various dates (UTC vs local edge case).
- File mode is `0o600`; dir mode is `0o700`.

**Integration tests** (`internal/entitymanager/` and `internal/scheduler/`):

- Each Manager write op produces exactly one record (table-driven over create/update/delete/rename for entities; create/update/delete for relations) using the `Memory` backend. Assert `Principal` flows from ctx.
- Automation-cascaded write produces a record with `triggered_by: automation:<name>` (in `internal/entitymanager/` since autocascade is exercised there via real automations). Cascade record inherits Principal from the originator.
- Scheduler-driven write (via fixture schedule + Lua script) produces a record with `triggered_by: schedule:<name>` and `Principal.Tool == "scheduler"` (in `internal/scheduler/`).
- Audit `Record` failing via a stub backend (Memory wrapped to emit slog.Error) still allows the write to succeed; slog warning is observable via a captured handler.
- `Nop` audit means writes succeed and no records are observable.

**Entry-point smoke tests:** for each of `internal/cli/`, `internal/mcp/`,
`cmd/rela-server`, `cmd/rela-desktop`, `internal/scheduler/` — a tiny test
that exercises the entry-point's ctx-bootstrapping code and asserts the
expected `Principal.Tool` literal lands on a subsequent Manager call.

**Edge Cases:**

- Audit dir doesn't exist on first write → created with mode 0o700.
- Audit dir exists but contains a stale file from a previous day → new day's file is created alongside.
- Concurrent first-writes on a fresh dir → MkdirAll is idempotent; no error.
- Process started just before midnight UTC → first record after midnight rotates correctly.
- `$USER` set to whitespace-only → `SystemUser` returns `"unknown"`.
- Long entity ID / type / triggered_by / Principal.User / Principal.Tool → length-capped at 1024 each.
- Empty `summary` → record valid; `summary` is optional.
- Disk full / read-only filesystem → backend self-logs the error via slog, write still succeeds (AC9).
- ctx without `WithPrincipal` → record carries `Principal{User: "unknown", Tool: "unknown"}` rather than panicking.

**Negative Tests:**

- `NewFilesystem("")` → returns error.
- `NewFilesystem(dir)` where the process can't create dir → first `Record` triggers slog warning, no panic.
- `entitymanager.New` with `Deps{... Audit: nil}` → returns error (AC11).
- `appbuild.Discover` without audit wired → returns error (AC11).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| Audit file growth is unbounded | Document under "operator concerns"; daily rotation gives natural retention granularity (`find .rela/audit -mtime +N -delete`). Automatic retention is out of scope. |
| Performance overhead per write (one fsync-less append) | Negligible vs. existing markdown serialization + git-friendly write. Benchmark only if write throughput becomes a concern. |
| Slog warning on audit failure is easy to miss in production | Document that operators should monitor `audit.write_failed` warning. Consider exposing a /healthz signal in a follow-up. |
| Schema change later breaks consumers parsing audit lines | Document the format as best-effort; JSON with optional fields is naturally forward-compatible. Principal lands as a sub-object now, so future fields (`principal.email`, `principal.session_id`) extend without breaking existing readers. |
| `context.Context` value is goroutine-safe but easy to forget to thread through | Mitigated by integration tests (AC5, AC6) that fail loudly if `triggered_by` is missing on a cascade, and entry-point smoke tests that catch a missing `WithPrincipal`. Any new write site that does *not* propagate ctx will produce a record with `Principal{User:"unknown", Tool:"unknown"}` — degraded but not broken. |
| Adding `Audit` to `entitymanager.Deps` ripples through every Manager construction call site | Minimal ripple by design: `Manager` is constructed in exactly two places — `appbuild.Discover` (production) and `appbuild.NewForTest` (tests). `NewForTest` defaults to `audit.Nop{}` so individual test files don't change. |
| Per-entry-point `WithPrincipal` stamping is N separate call sites, easy to miss one | Mitigated by entry-point smoke tests (one per binary / root command). Missing a stamp degrades to `Tool:"unknown"` — visible in audit log, not silent. |

**Effort:** **m** (bumped slightly from original "m" — Principal type +
per-entry-point stamping adds ~½ day). Rough breakdown: ~½ day audit
package (Principal, Record, Nop, Memory, Filesystem, context helpers,
SystemUser), ~½ day entitymanager.Deps + recordAudit wiring + nil-rejection,
~½ day autocascade/scheduler context wrapping + per-entry-point stamping,
~1 day tests + integration + entry-point smoke tests, ~½ day docs + cleanup.
**The "helper-first" caveat in IMPL-XYKA no longer applies** — the workspace
decomposition already produced the single-chokepoint test fixture
(`appbuild.NewForTest`).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — short page describing `.rela/audit/` location, JSONL record shape (including `Principal` sub-object), and operator concerns (manual rotation/retention; how to set `$USER`).
- [ ] CLI help text — N/A: no new commands.
- [x] CLAUDE.md — note that any new write path through `entitymanager.Manager` is automatically audited (no extra wiring required), that new entry-point binaries must stamp `audit.WithPrincipal(ctx, Principal{User: audit.SystemUser(), Tool: "..."})` at startup, and that engine-initiated callers should propagate `audit.WithTriggeredBy(ctx, ...)` through their `context.Context`.
- [ ] README.md — N/A.
- [ ] API docs — N/A.
- [ ] N/A.

## Design Review

- [x] ~~Run `/design-review`~~ (Skipped: plan reviewed via `/crit` instead — see Findings below; all addressed.)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Reviewed via `crit` (rounds 1–2). Round 1 raised
three findings, all addressed in this revision: r_5c51f4 (cleanup of
train-of-thought writing), c_c3d93c (pluggable backend system: Nop / Filesystem
/ Memory), c_324c00 (use `context.Context` instead of goroutine-local state for
triggered-by). Round 2 finished with no new findings.

**Post-decomposition update (2026-05-17):** plan rewritten against current
package layout (workspace → entitymanager + appbuild). Substance unchanged;
acceptance criteria unchanged.

**Crit round 2 (2026-05-17):** three findings addressed.

- **c_45b5a5** (don't try-hard actor resolution): dropped `$RELA_ACTOR` and `git config user.email` from the fallback chain. `audit.SystemUser()` is now just `$USER → "unknown"`. Operators wanting a custom identity set `$USER`; data-entry will support per-request override via header in a follow-up.
- **c_4b2baf** (Principal type in this PR, not later): added `audit.Principal{User, Tool}` as a first-class type. `Tool` is set by each entry point (cli / mcp / data-entry / scheduler / desktop). Plumbed via `context.Context` (`audit.WithPrincipal` / `PrincipalFrom`). New AC3 / AC4 cover this. Out-of-scope item "Structured Principal type" removed from the scope table.
- **c_717fd7** (any Go lib with flexible sinks?): researched. `slog.JSONHandler` + `lumberjack` rotates by size not date, hardcodes wrong file modes (0o644 / 0o744), and forces `level`/`msg` keys onto the schema. `rotoslog` uses a different filename pattern. Hand-rolled `encoding/json` + `os.OpenFile` is ~80 lines with zero deps and exact control — that decision stands. Added the full rationale to the Research section.
