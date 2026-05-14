---
id: PLAN-XKMJ
type: planning-checklist
title: 'Planning: Audit log: append-only JSONL of entity write operations'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In:**

- New `internal/audit` package with a small `Audit` interface and a pluggable backend system.
  - **`Nop`** — no-op, used by tests and explicit opt-out paths.
  - **`Filesystem`** — JSONL writer under `.rela/audit/YYYY-MM-DD.jsonl`, daily UTC rotation, append-only, internal mutex.
  - **`Memory`** — in-memory ring/slice backend used in integration tests so assertions can read records back without filesystem fixtures.
- `Audit` field on `lua.WriteDeps`. Production wiring through `internal/workspace/services.go` `LuaWriteDeps()`; test sites pass `audit.NewMemory()` or `audit.Nop{}`.
- EntityManager's workspace adapter (`internal/workspace/manager.go`) calls `Audit.Record` on every successful entity create/update/delete/rename and relation create/update/delete/update — once per op, after the store write returns.
- Best-effort `actor` resolution at process startup: `$RELA_ACTOR` (escape hatch for ops) → `$USER` → `git config user.email` → `"system"`. Resolved once per process and cached on the audit instance.
- `triggered_by` populated for engine-initiated writes via `context.Context`: `automation:<name>`, `schedule:<task-name>`. Direct user writes carry empty `triggered_by`. EntityManager interface methods already take `context.Context` (see `internal/entitymanager/entitymanager.go:101`); the value just isn't read today.
- Constructors `audit.NewFilesystem(...)` / `audit.NewMemory(...)` reject empty/invalid required fields per project's "constructors reject nil required fields" rule.

**Out (subsequent phases):**

- Structured `Principal` type. `actor` stays a string here.
- Write-policy hook (the EntityManager dispatch this ticket establishes will be reused by it).
- `outcome: denied` records — no policy yet means no denials. Schema admits optional `outcome` later.
- UI rendering, CLI helpers (`rela audit tail`), search/query over audit data.
- Read-side audit (no logging of reads).
- Log retention/cleanup policy. Documented as an operator concern.

**Decisions (confirmed with user):**

1. **No `outcome` field in v1.** Nothing to record outcome of yet — every write that reaches the audit hook is by definition successful. When write-policy lands, the schema gains an optional `outcome: "denied"` field; absence continues to mean success.
2. **Audit failure does not block the write.** `slog.Error` and continue. No retry, no buffering, no propagation up the write path. Audit is forensic; an unwritable `.rela/audit/` directory (disk full, perms) shouldn't refuse legitimate edits. Operators monitor for `audit.write_failed` log events.
3. **Automation cascades attribute to the innermost automation.** Matches what `LuaToExecute.AutomationName` already carries; matches "who is asking right now" semantics.

**Acceptance Criteria:**

1. **AC1: every entity write produces one audit record.** Create, update, delete, rename — for each, exactly one record reaches the configured backend. Table-driven test with the `Memory` backend.
2. **AC2: every relation write produces one audit record.** Same shape for create/update/delete relation.
3. **AC3: actor falls back through the chain.** Test by constructing the audit with each tier of inputs available/unavailable; assert resolved actor matches the expected fallback.
4. **AC4: triggered_by populated for automation-driven writes.** Integration test: metamodel with an `on: created` automation that creates a child entity. Triggering write produces two records — the user write with empty `triggered_by`, the cascade with `triggered_by: "automation:<name>"`.
5. **AC5: triggered_by populated for scheduler-driven writes.** Integration test: scheduler runs a Lua task that calls `rela.create_entity`; resulting record carries `triggered_by: "schedule:<task-name>"`.
6. **AC6: daily rotation.** With an injected clock, first record lands in `2026-05-10.jsonl`, second (across the boundary) in `2026-05-11.jsonl`.
7. **AC7: append-only correctness under concurrency.** N parallel writes through EntityManager produce N intact JSONL lines. (Workspace's writer mutex makes this trivial; tested explicitly with `-race`.)
8. **AC8: audit failure does not block the write.** A backend whose write returns an error still allows the entity write to succeed; a `slog.Error` is emitted (`audit.write_failed`). No retry, no buffering.
9. **AC9: Nop is safe.** `WriteDeps{Audit: audit.Nop{}}` works without panic and without writing anything.
10. **AC10: nil Audit is rejected at construction.** Workspace constructor validates that an `Audit` is supplied; tests with empty `WriteDeps{}` either set `Nop{}` or fail loudly. (Per project rule: constructors reject nil required fields. We intentionally do *not* paper over a missing audit by silently no-oping.)

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Stdlib only.** JSONL is `encoding/json` + `os.OpenFile(O_APPEND|O_CREATE|O_WRONLY)`. Existing audit libraries (`audit-go`, OPA decision logs) are RBAC/policy-shaped — adopting one now would force premature schema decisions.
- **slog is the project's structured-logging standard** (`.golangci.yml` enforces `log/slog`). Audit JSONL records align field-naming conventions with slog (`time` not `timestamp`, lower-snake-case keys) for visual consistency, but write through a dedicated writer rather than via slog because:
  - Audit needs append-on-disk semantics with a fixed file layout, not a generic log handler.
  - Mixing audit with operational logs would make `tail -f` either too noisy or require fragile filtering.
- **`.rela/scheduler-state.json`** (`internal/scheduler/scheduler.go`) is the closest existing "engine writes JSON under .rela/" pattern — same `filepath.Join(paths.CacheDir, ...)` construction. Audit follows that.

**Reference patterns in codebase:**

- `internal/entitymanager/entitymanager.go:101–123` — every write method already takes `context.Context`. The workspace adapter (`internal/workspace/manager.go`) currently ignores it (`_ context.Context`). This is the natural carrier for `triggered_by`.
- `internal/workspace/services.go:38–43` — `LuaWriteDeps()` is the single production builder for `WriteDeps`. Wiring `Audit` here covers dataentry, MCP, scheduler, CLI in one stroke.
- `internal/scheduler/scheduler.go:138` — `engine.ExecuteFile(...)` is the single scheduler entry point; `task.Name` is in scope. Wrap the call's `context.Context` with the schedule name.
- `internal/automation/engine.go:233` — `executeAction(...)` already has `automationName` in scope and stamps it onto `LuaToExecute.AutomationName`. The workspace then runs the Lua; that's where the context value is set.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. **Package `internal/audit`:**

   ```go
   type Record struct {
       Time        time.Time `json:"time"`
       Op          string    `json:"op"`
       EntityType  string    `json:"entity_type"`
       EntityID    string    `json:"entity_id"`
       Actor       string    `json:"actor"`
       TriggeredBy string    `json:"triggered_by,omitempty"`
       Summary     string    `json:"summary,omitempty"`
   }

   type Audit interface { Record(rec Record) }

   type Nop struct{}
   type Memory struct{ /* mutex + records */ }
   type Filesystem struct{ /* dir, actor, mutex, current file/date */ }

   func NewFilesystem(dir, actor string) (*Filesystem, error)
   func NewMemory() *Memory
   func ResolveActor() string
   ```

The `Audit` interface is single-method and consumer-side, per CLAUDE.md.
`Memory.Records()` returns a snapshot for test assertions.

2. **`triggered_by` plumbing via `context.Context`** — the existing interface already carries `ctx`; we just start *using* it.

   ```go
   // internal/audit/context.go
   type triggeredByKey struct{}

   func WithTriggeredBy(ctx context.Context, label string) context.Context {
       return context.WithValue(ctx, triggeredByKey{}, label)
   }

   func TriggeredByFrom(ctx context.Context) string {
       if v, ok := ctx.Value(triggeredByKey{}).(string); ok { return v }
       return ""
   }
   ```

The workspace adapter reads `audit.TriggeredByFrom(ctx)` on each write and
includes it in the `Record`. No new field on `WriteDeps`. No goroutine-local
state. No mutex dance.

3. **EntityManager hooks.** The workspace's `wsEntityManager` adapter is the single chokepoint with all 7 write methods. Add:

   ```go
   func (m *wsEntityManager) recordAudit(ctx context.Context, op, entityType, entityID, summary string)
   ```

Invoke on each method's tail-success branch. The adapter is constructed from the
workspace, which holds the `Audit` instance.

4. **Automation engine plumbing.** When the workspace runs Lua emitted by an automation (`workspace.go` automation execution path), derive a per-execution context with `audit.WithTriggeredBy(ctx, "automation:"+luaToExec.AutomationName)` and pass it through to the EntityManager calls that Lua makes. For non-Lua automation actions (Set, CreateRelation, CreateEntity) executed directly by the workspace on behalf of an automation, do the same wrapping at the call site.

5. **Scheduler plumbing.** `internal/scheduler/scheduler.go:138` derives `ctx := audit.WithTriggeredBy(parent, "schedule:"+task.Name)` and threads it through the script execution path.

6. **Production wiring.**
   - Each command entry point (`cmd/rela/...`, `cmd/rela-server/...`, `cmd/rela-desktop/...`) constructs `audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"), audit.ResolveActor())` and injects it into the workspace.
   - Workspace constructor takes `Audit` as a required collaborator and rejects nil.
   - `LuaWriteDeps()` propagates the workspace's `Audit` into `WriteDeps`.

**Files to modify:**

- **New:** `internal/audit/audit.go` (interface, Record), `internal/audit/nop.go`, `internal/audit/memory.go`, `internal/audit/filesystem.go`, `internal/audit/context.go`, `internal/audit/actor.go`, plus `_test.go` for each.
- `internal/lua/deps.go` — add `Audit audit.Audit` to `WriteDeps`.
- `internal/workspace/services.go` — `LuaWriteDeps()` sets `Audit`. Workspace constructor takes/holds an `Audit`.
- `internal/workspace/workspace.go` — wrap automation-execution paths with `audit.WithTriggeredBy(ctx, "automation:"+name)`.
- `internal/workspace/manager.go` — `wsEntityManager` calls `recordAudit(ctx, ...)` on each write success path; reads `audit.TriggeredByFrom(ctx)`.
- `internal/scheduler/scheduler.go` — wrap script execution context with `audit.WithTriggeredBy(ctx, "schedule:"+task.Name)`.
- `internal/dataentry/app.go` — pass an `Audit` to workspace constructor.
- `cmd/rela/...`, `cmd/rela-server/...`, `cmd/rela-desktop/...` — construct `audit.NewFilesystem(...)` and inject (one new line per entry point).
- Test files: `WriteDeps{}` literals get `Audit: audit.NewMemory()` or `Audit: audit.Nop{}` based on whether the test asserts on records.
- A docs page covering `.rela/audit/` location, record shape, and operator concerns (manual rotation/retention).

**Alternatives considered (rejected):**

- **Bolt audit into the store layer.** Multiple stores (memstore, fsstore) each get instrumented and tests for each backend drag in audit concerns. EntityManager is already the chokepoint for "human intent" — the right layer.
- **Use slog with a JSONHandler writing to `.rela/audit/...`.** Ties wire format to slog's record encoding (forces `level`, `msg` fields), couples to slog handler lifecycle, and doesn't naturally express daily rotation.
- **Async audit (channel + worker).** Synchronous writes are simple, correctness-obvious, and adequate given write throughput is low and already serialized. Async can be added later behind the same interface.
- **`WriteContext` field on `WriteDeps`** for triggered-by. Rejected once it became clear the EntityManager interface already takes `context.Context` end-to-end. Adding a parallel state-passing channel would be redundant.
- **Goroutine-local / mutex-swap state** for triggered-by. Rejected for the same reason — `context.Context` is Go's idiom and is already plumbed.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **`$RELA_ACTOR` env var** — operator-controlled. Trimmed; capped at 256 chars; printable UTF-8 only (control chars stripped). Invalid → fall through to `$USER`.
- **`$USER` env var** — same treatment.
- **`git config user.email`** — invoked via `exec.Command("git", "config", "user.email")` with explicit args (no shell). 2-second timeout. Any error → fall through to `"system"`. Output trimmed and length-capped.
- **Entity IDs / types** — already validated upstream; we stringify and let JSON encoding handle escaping.
- **`triggered_by` value** — sourced from metamodel-loaded automation/schedule names (validated at load). Length-capped + control-char stripped at write time as defense-in-depth against a metamodel author putting a newline in a name and corrupting the JSONL stream.

**Security-Sensitive Operations:**

- **File creation under `.rela/audit/`** — `os.MkdirAll(dir, 0o700)`, files opened with `0o600`. Path is `filepath.Join(cacheDir, "audit", date+".jsonl")` where `date` is `time.Now().UTC().Format("2006-01-02")` — no user input enters the path.
- **Actor resolution does not panic on missing git** — subprocess failure is non-fatal.
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

- Record JSON round-trip: marshal/unmarshal, omitempty behavior.
- `NewFilesystem` rejects empty dir, empty actor.
- `NewMemory` returns a working backend; `Records()` is a snapshot (mutating it doesn't affect the backend).
- Daily rotation across an injected clock boundary.
- Concurrent `Record` calls produce N intact JSONL lines (no interleaving).
- Actor resolution chain across each tier.
- `WithTriggeredBy` / `TriggeredByFrom` round-trip; absence returns empty string.
- Filename construction for various dates (UTC vs local edge case).
- File mode is `0o600`; dir mode is `0o700`.

**Integration tests** (`internal/workspace/`):

- Each EntityManager write op produces exactly one record (table-driven over create/update/delete/rename for entities; create/update/delete for relations) using the `Memory` backend.
- Automation-cascaded write produces a record with `triggered_by: automation:<name>`.
- Scheduler-driven write (via fixture schedule + Lua script) produces a record with `triggered_by: schedule:<name>`.
- Audit `Record` returning error (via stub backend that wraps Memory and returns failures) still allows the write to succeed; slog warning is observable via a captured handler.
- `Nop` audit means writes succeed and no records are observable.

**Edge Cases:**

- Audit dir doesn't exist on first write → created with mode 0o700.
- Audit dir exists but contains a stale file from a previous day → new day's file is created alongside.
- Concurrent first-writes on a fresh dir → MkdirAll is idempotent; no error.
- Process started just before midnight UTC → first record after midnight rotates correctly.
- `$USER`, `$RELA_ACTOR` set to whitespace-only → treated as empty, fall through.
- Long entity ID / type / triggered_by → length-capped at 1024 each.
- Empty `summary` → record valid; `summary` is optional.
- Disk full / read-only filesystem → `Record` swallows the error, slog.Error, write still succeeds (AC8).

**Negative Tests:**

- `NewFilesystem("", "x")` → returns error.
- `NewFilesystem("x", "")` → returns error.
- `NewFilesystem("x", "y")` then writing into a path the process can't create → first `Record` triggers slog warning, no panic.
- Workspace constructor with nil Audit → returns error (AC10).

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
| Schema change later (e.g., principal) breaks consumers parsing audit lines | Document the format as best-effort; JSON with optional fields is naturally forward-compatible. Field semantics won't change; new fields may be added. |
| `context.Context` value is goroutine-safe but easy to forget to thread through | Mitigated by integration tests (AC4, AC5) that fail loudly if `triggered_by` is missing on a cascade. Also: any new write site that does *not* propagate ctx will produce a record with empty `triggered_by` — degraded but not broken. |

**Effort:** **m** (matches the ticket effort field). Rough breakdown: ~½ day
audit package (interface + Nop + Memory + Filesystem + context helpers + actor
resolution), ~½ day workspace/services wiring + nil-rejection, ~½ day
automation/scheduler context wrapping, ~1 day tests + integration, ~½ day docs +
cleanup.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — short page describing `.rela/audit/` location, JSONL record shape, and operator concerns (manual rotation/retention).
- [ ] CLI help text — N/A: no new commands.
- [x] CLAUDE.md — note that any new write path through EntityManager is automatically audited (no extra wiring required) and that engine-initiated callers should propagate `audit.WithTriggeredBy(ctx, ...)` through their `context.Context`.
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
