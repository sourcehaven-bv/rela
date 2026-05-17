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

1. **AC1: every entity write produces one audit record with the right Subject.** Create / update / delete each produce one record with `Subject = {kind:"entity", type, id}`. Rename produces one record with `Before` and `After` both populated (`Subject` zero). Table-driven test in `internal/entitymanager/` using the `Memory` backend.
2. **AC2: every relation write produces one audit record with the right Subject.** Create / update / delete each produce one record with `Subject = {kind:"relation", relation_type, from_id, to_id}`.
3. **AC3: Principal flows from ctx into the record.** Set `audit.WithPrincipal(ctx, Principal{User:"alice", Tool: audit.ToolCLI})` → record carries `principal.user="alice"`, `principal.tool="cli"`. Absence → `Principal{User:"unknown", Tool:"unknown"}`.
4. **AC4: each entry point stamps the right Tool.** Entry-point smoke tests (one per: cli root, mcp server, data-entry server, scheduler, desktop) confirm `Principal.Tool` is the expected `ToolXxx` constant. Data-entry stamps `User: "unknown"` per the design-review decision; mcp stamps `User: audit.SystemUser()` of the host process.
5. **AC5: triggered_by populated for automation-driven writes.** Integration test: metamodel with an `on: created` automation that creates a child entity. Trigger produces ≥2 records — the user write with empty `triggered_by`, the cascade with `triggered_by: "automation:<name>"`. All records inherit the originator's Principal.
6. **AC6: triggered_by populated for scheduler-driven writes.** Integration test in `internal/scheduler/`: scheduler runs a Lua task that calls `rela.create_entity`; resulting record carries `triggered_by: "schedule:<task-name>"` and `principal.tool="scheduler"`.
7. **AC7: delete-cascade produces 1+N records.** DeleteEntity with `cascade=true` on an entity with N incident relations produces 1 entity-delete record + N relation-delete records. Each relation-delete record carries `triggered_by: "cascade:delete-entity:<root-id>"`.
8. **AC8: daily rotation under an injected clock.** With `audit.WithClock(...)`, first record at 23:59 UTC lands in `2026-05-10.jsonl`, second at 00:01 UTC (next day) lands in `2026-05-11.jsonl`. Both files exist; first file is closed and unchanged after the rotation.
9. **AC9: append-only correctness under concurrency including rotation race.** N parallel writes through Manager, with the injected clock crossing midnight during the burst, produce N intact JSONL lines distributed correctly across two files. (`Filesystem` carries an internal mutex over date-check + open + append.) Run with `-race`.
10. **AC10: audit failure does not block the write.** A Filesystem whose dir is a symlink (or `O_NOFOLLOW` open fails) is silently downgraded to a Nop and the entity write succeeds. `slog.Error("audit.write_failed", ...)` is emitted. Captured slog handler asserts this in tests.
11. **AC11: Nop is safe and Memory is observable.** `entitymanager.Deps{... Audit: audit.Nop{}}` works without panic or write. `Memory` records all writes and `Records()` returns a snapshot.
12. **AC12: nil Audit is rejected at construction.** `entitymanager.New` validates `Deps.Audit != nil` and returns an error. `appbuild.New` validates the same. (Discover constructs Audit, so its own nil-check would be unreachable — guarded at the lower layer.) `appbuild.NewForTest` carve-out: substitutes `audit.Nop{}` when no `WithTestAudit` is supplied.
13. **AC13: Lua scripts cannot rewrite their own attribution.** Negative test: a Lua writer runtime built from `lua.NewWriter(deps)` exposes no `audit` table on `rela.*`. A Lua script calling `rela.audit.with_principal(...)` (or any path that could swap principal) errors out — the binding doesn't exist.
14. **AC14: symlinked audit dir is refused.** Stat the audit dir; if it's a symlink, the Filesystem backend's open fails on first write, slog.Error fires, audit becomes Nop for the rest of the process. Entity writes succeed regardless.
15. **AC15: control chars in Record fields are sanitized at the JSONL boundary.** A Record containing `\x00` / `\n` / `\x1b` in any string field round-trips to the Filesystem as printable text only; `jq` consumers see clean lines. Memory backend retains the raw bytes (test asymmetry documented).

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
   // identifies the entry point: one of the ToolXxx constants below.
   type Principal struct {
       User string `json:"user"`
       Tool string `json:"tool"`
   }

   // Tool constants — referenced by entry-point wiring so typos surface
   // at compile time. Adding a new tool means adding a constant here.
   const (
       ToolCLI        = "cli"
       ToolMCP        = "mcp"
       ToolDataEntry  = "data-entry"
       ToolScheduler  = "scheduler"
       ToolDesktop    = "desktop"
   )

   // Subject identifies what the op acted on. Exactly one of Entity /
   // Relation is set per record; readers switch on Kind.
   //
   //   - entity:   {kind:"entity",   type, id}
   //   - relation: {kind:"relation", relation_type, from_id, to_id}
   type Subject struct {
       Kind         string `json:"kind"`                    // "entity" | "relation"
       Type         string `json:"type,omitempty"`          // entity only
       ID           string `json:"id,omitempty"`            // entity only
       RelationType string `json:"relation_type,omitempty"` // relation only
       FromID       string `json:"from_id,omitempty"`       // relation only
       ToID         string `json:"to_id,omitempty"`         // relation only
   }

   // Record is one audit row. For rename ops, Before and After are
   // both populated (and Subject is left zero) so readers can answer
   // "what was X renamed to?". For every other op exactly Subject is
   // populated and Before/After are zero.
   type Record struct {
       Time        time.Time `json:"time"`
       Op          string    `json:"op"`     // see Op* constants
       Subject     Subject   `json:"subject,omitempty"`
       Before      Subject   `json:"before,omitempty"` // rename only
       After       Subject   `json:"after,omitempty"`  // rename only
       Principal   Principal `json:"principal"`
       TriggeredBy string    `json:"triggered_by,omitempty"`
       Summary     string    `json:"summary,omitempty"`
   }

   // Op* constants — the values that appear in Record.Op. Stable wire
   // contract.
   const (
       OpCreateEntity   = "create-entity"
       OpUpdateEntity   = "update-entity"
       OpDeleteEntity   = "delete-entity"
       OpRenameEntity   = "rename-entity"
       OpCreateRelation = "create-relation"
       OpUpdateRelation = "update-relation"
       OpDeleteRelation = "delete-relation"
   )

   type Audit interface { Record(rec Record) }

   type Nop struct{}
   type Memory struct{ /* mutex + records */ }
   type Filesystem struct{ /* dir, mutex, current file/date, clock */ }

   func NewFilesystem(dir string, opts ...Option) (*Filesystem, error)
   func NewMemory() *Memory

   // WithClock injects a clock for AC7 (rotation testing). Production
   // wiring omits this and gets time.Now().UTC.
   type Option func(*filesystemConfig)
   func WithClock(now func() time.Time) Option

   // SystemUser resolves the OS user at process start: $USER → "unknown".
   func SystemUser() string
   ```

   The `Audit` interface is single-method and consumer-side, per CLAUDE.md.
   `Memory.Records()` returns a snapshot for test assertions. The
   `Filesystem` backend opens files with `O_APPEND|O_CREATE|O_WRONLY|O_NOFOLLOW`
   and `lstat`s the audit dir on each open to refuse symlinks (defense against
   redirect-to-elsewhere attacks; documented in Security Considerations).

**Summary-string policy** (one per Op, computed by Manager's `recordAudit`
helper):

| Op | Summary |
|---|---|
| `create-entity` | `"created"` |
| `update-entity` | `"updated: <comma-separated changed property names>"` (no values; defense against secret leakage) |
| `delete-entity` | `"deleted"` (cascade: counts of cascaded relations appear in `summary` of the cascade records, not the trigger) |
| `rename-entity` | `"renamed"` (Before/After carry the identity diff) |
| `create-relation` | `"created"` |
| `update-relation` | `"updated: <comma-separated changed meta keys>"` |
| `delete-relation` | `"deleted"` |

A single policy applied everywhere keeps the log machine-parseable. If a
caller needs richer per-op metadata later it goes into a separate field, not
the freeform Summary.

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

3. **Manager hooks.** Two private helpers — one per Subject shape:

   ```go
   func (m *Manager) recordEntityAudit(
       ctx context.Context, op string, e *entity.Entity, summary string,
   )
   func (m *Manager) recordRelationAudit(
       ctx context.Context, op string, r *entity.Relation, summary string,
   )
   func (m *Manager) recordRenameAudit(
       ctx context.Context, before, after *entity.Entity,
   )
   ```

   Each helper reads `audit.PrincipalFrom(ctx)`, `audit.TriggeredByFrom(ctx)`,
   stamps `Time: time.Now().UTC()`, and calls `m.deps.Audit.Record(...)`.
   Invoked on each of the 7 write methods' tail-success branch.

   **Delete-cascade semantics:** `DeleteEntity` with `cascade=true` removes
   the entity plus N incident relations. Each persisted mutation produces
   one record:
   - 1 record for the entity delete (`op: delete-entity`, summary
     `"deleted (cascade)"` when cascade=true).
   - N records for the cascaded relation deletes, each carrying
     `triggered_by: "cascade:delete-entity:<id>"`.

   The cascade records' Principal is inherited from the same ctx — same human
   ultimately caused the relation deletions.

4. **Crash window — accepted and documented.** A process crash between the
   `store.Store` write returning success and `Audit.Record` returning leaves
   an unaudited mutation on disk. The plan accepts this gap explicitly:
   - Audit is forensic, not authoritative. The store is the source of truth.
   - Closing the window via audit-before-commit creates a worse failure mode
     (false-positive audit rows for writes that never landed).
   - Adding fsync per audit append is rejected for v1 (adds disk latency on
     every write). Operators wanting stronger guarantees can fsync via
     filesystem mount options or move to an external append-only sink in a
     future phase.
   - Documented in `docs/.../audit.md` so this is a *known* gap, not a
     surprise during an incident.

5. **Autocascade plumbing.** `autocascade.Runner.executeScriptActions`
   (`internal/autocascade/runner.go:243-296`) is the precise insertion point:
   immediately before `scripts.Run(ctx, ...)` on line 268, derive

   ```go
   actionCtx := audit.WithTriggeredBy(ctx, "automation:"+action.AutomationName)
   ```

   and pass `actionCtx` to `scripts.Run`. The non-script cascade paths —
   `applyRelationCreations` (line 207) and `processEntityCreations` — call
   into `host.WriteRelation` / `host.CreateEntity` / etc. directly; those
   call sites also need their ctx wrapped with the same triggered-by label
   so cascade-created relations and entities are attributed to the
   automation that produced them.

   Principal is *not* overwritten — the cascade inherits the originator's
   Principal, which is the desired attribution.

6. **Scheduler plumbing.** `internal/scheduler/scheduler.go` derives:

   ```go
   ctx := audit.WithPrincipal(parent, audit.Principal{
       User: audit.SystemUser(), Tool: audit.ToolScheduler,
   })
   ctx = audit.WithTriggeredBy(ctx, "schedule:"+task.Name)
   ```

   immediately before invoking the script engine. Lua bindings call Manager
   through the same ctx; both values flow naturally.

7. **Per-entry-point Principal stamping.** Each entry point binary or root
   command attaches `Principal` once:

   | Entry point | Tool | User |
   |---|---|---|
   | `internal/cli/root.go` (cmd/rela, cmd/rela-server when invoked CLI-side) | `ToolCLI` | `audit.SystemUser()` (= `$USER`) |
   | `cmd/rela-desktop` | `ToolDesktop` | `audit.SystemUser()` |
   | `internal/scheduler` | `ToolScheduler` | `audit.SystemUser()` (step 6) |
   | `internal/mcp` server entry | `ToolMCP` | `audit.SystemUser()` of the *host process* — **not** the LLM caller |
   | `cmd/rela-server` (HTTP serving mode) | `ToolDataEntry` | `"unknown"` (intentional — see below) |

   **Why data-entry uses `User: "unknown"` rather than the server process
   user (`www-data`, container user, etc.):** the process-user value would
   be recorded for every edit by every human user of the web app, which is
   actively misleading — operators reading the audit log would conclude
   "alice's process made this change" when in reality alice is the
   *operator running the server*, not the user making edits. Stamping
   `"unknown"` is honest: per-request user attribution lands in a follow-up
   PR via an HTTP middleware that reads a header / cookie / session and
   overrides `Principal.User` per-request.

   **Why MCP stamps the host-process user, not the LLM:** MCP's wire
   protocol carries no notion of a "user" — it's stdio-bound to a single
   client. The host process user is the *operator* who launched
   `rela mcp ...`. The audit log records "alice ran an MCP-backed agent
   that did X", which is the right grain for forensics. If a future MCP
   variant transports principal-information per-call, an HTTP middleware
   analogue applies; not in scope here.

   **Lua-eval inheritance:** `rela lua-eval <file>` and other Lua-driven
   CLI commands inherit `ToolCLI`. In v1, lua-eval writes are
   indistinguishable from native CLI writes; a follow-up can introduce
   `Tool: "lua-eval"` or a per-call sub-tool if forensic granularity
   demands it.

8. **Lua-binding hardening — Principal/TriggeredBy not exposed.** `audit.WithPrincipal` and `audit.WithTriggeredBy` are Go-only. They are **not** registered on the Lua `rela.*` table, and the Lua write bindings do *not* construct their own contexts — they pass through the ctx supplied by the script runner. This means a Lua script cannot rewrite its own attribution. Enforced by:
   - No `rela.audit` Lua binding exists (negative test: `lua.NewWriter(deps)` exposes no `audit` table).
   - The Lua script-runner adapter (`internal/script/luascriptrunner.go`) passes the supplied ctx through verbatim — does not `context.Background()` away the parent ctx, does not strip principal-key values.

9. **Production wiring.**
   - `appbuild.Discover` constructs `audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))` and passes it into `entitymanager.New` via `Deps.Audit`.
   - `appbuild.New` (the lower-level constructor) requires `Audit` and rejects nil. **AC11 wording corrected:** Discover *constructs* Audit so it can't observe nil; the nil-check guards `appbuild.New` (used today only by `appbuild.NewForTest`, but treated as a public constructor for future custom-wiring callers). The check is defensive but not dead — it documents the contract on `Deps.Audit`.
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

- **`$USER` env var** (via `audit.SystemUser()`) — trimmed; sanitized (see policy below). Empty or invalid → `"unknown"`. No fancier chain (originally proposed `$RELA_ACTOR` + git fallback; dropped as over-engineered).
- **`Principal.Tool`** — set by the wiring site from a fixed allowlist via the `ToolXxx` constants. Not user-controlled. Typed constants surface entry-point typos at compile time; an entry-point smoke test enumerates expected values.
- **Entity IDs / types / relation types** — already validated upstream by the metamodel (allowlist: lowercase alphanumeric + hyphen + underscore). No untrusted input enters Subject fields.
- **`triggered_by` value** — sourced from metamodel-loaded automation/schedule names (validated at load against the same allowlist) or constructed by audit itself (`"cascade:delete-entity:<id>"` — `<id>` is metamodel-validated).
- **Summary string** — constructed entirely by Manager from validated property/meta names. No property *values* are interpolated.

**Sanitization policy — single layer at the Filesystem backend:** all string
fields on `Record` are sanitized in `Filesystem.Record` before
`encoding/json.Marshal`:

1. Truncate to 1024 chars per field (UTF-8 safe).
2. Replace `\x00-\x1f` and `\x7f` (C0/DEL) with ` ` (non-breaking
   space). Leaves printable UTF-8 untouched.

The sanitization is concentrated at one site (the backend) — Memory and Nop
do not sanitize (Memory holds the raw Record for test assertions; Nop is a
no-op). This means a misbehaving caller can put a `\n` in a Subject.ID and
the Memory backend will see it; the JSONL file will not. Documented as a
known asymmetry — the rationale is that the JSONL stream is the threat
surface (downstream `jq` / `tail -f` consumers) and Memory is test-only.

**Security-Sensitive Operations:**

- **File creation under `.rela/audit/`:**
  - `os.MkdirAll(dir, 0o700)`; files opened with `os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY|syscall.O_NOFOLLOW, 0o600)`. `O_NOFOLLOW` refuses to open if the final path component is a symlink.
  - Before MkdirAll, `os.Lstat(dir)` checks: if dir exists and is a symlink, refuse to use it (slog.Error + audit becomes a Nop for the rest of the process). Refusing-and-degrading is preferred over panicking in the audit path.
  - Path is `filepath.Join(cacheDir, "audit", date+".jsonl")` where `date` is `time.Now().UTC().Format("2006-01-02")` — no user input enters the path.
- **Midnight rotation algorithm (under the Filesystem mutex):**
  1. On each `Record(rec)` call, compute `today := clock().UTC().Format("2006-01-02")`.
  2. If `today != f.currentDate` (or `f.file == nil`): close the existing file (if open), open the new file under `today`'s name, update `f.currentDate`.
  3. Append `json.Marshal(rec) + "\n"`.

  All three steps run under `f.mu` so a concurrent writer can never observe
  a half-rotated state. AC7 (`-race`) covers concurrent rotation crossing.
- **Audit records can contain entity IDs and types** — these are not secrets in rela's model (entities are markdown files in the project tree).
- **No property values logged** in v1's `summary`. Only `"created"`, `"updated: status,priority"`, `"deleted"`. Property *names* may appear; values do not — defense against secrets accidentally stored in properties.

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

- [x] Ran `/design-review` (cranky-code-reviewer pass, 2026-05-17)
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

**Design-review pass (2026-05-17):** cranky-code-reviewer surfaced 14
findings (4 critical, 9 significant, 1 minor). All critical and significant
findings addressed before implementation:

- **CRIT — Record can't identify a relation; rename loses the old identity:** Schema redesigned. `Record.Subject` is now polymorphic (`Subject{Kind, Type, ID, RelationType, FromID, ToID}`). Rename adds `Before` / `After` Subject pair. Op constants added.
- **CRIT — Delete-cascade semantics:** Settled. DeleteEntity(cascade=true) on N relations produces 1+N records; cascade records carry `triggered_by: "cascade:delete-entity:<id>"`. New AC7.
- **CRIT — Crash window between store-write and audit-append:** Accepted and documented. Audit is forensic, not authoritative; closing the window via audit-before-commit creates worse failure modes. Documented in step 4 of Technical Approach and in user-facing audit docs.
- **SIG — JSONL stream corruption / control-char policy:** Concentrated at the Filesystem backend. C0 + DEL replaced with non-breaking space; truncate at 1024 chars. Memory backend retains raw bytes (test asymmetry documented). New AC15.
- **SIG — Symlink / TOCTOU on `.rela/audit/`:** `O_NOFOLLOW` on file open, `Lstat` on dir before MkdirAll. Symlinked dir → audit degrades to Nop, slog.Error. New AC14.
- **SIG — Midnight rotation algorithm + injected clock:** Algorithm specified (check-then-open under the same mutex). `audit.WithClock(...)` option added to type sketch. New AC8 / AC9.
- **SIG — Data-entry principal is misleading:** Resolved by stamping `User: "unknown"` for the data-entry server, not the process owner. Per-request override is the explicit follow-up. New AC4 verifies this.
- **SIG — Principal spoofing from Lua:** Documented and tested. `audit.WithPrincipal` is not exposed in the Lua API; the script-runner adapter passes ctx through verbatim. New AC13.
- **SIG — `appbuild.New` nil-check potentially dead:** Documented as defensive-but-not-dead — guards future custom-wiring callers; Discover constructs Audit so its own check would be unreachable. AC12 wording corrected.
- **SIG — Summary policy for 7 ops:** Table added to step 1 of Technical Approach. One policy applied everywhere; per-op richer metadata goes into separate fields, not freeform Summary.
- **SIG — Autocascade insertion point research-still-needed:** Resolved. Read `internal/autocascade/runner.go` directly; insertion point is line 268 (before `scripts.Run`), with parallel wraps at line 207 (`applyRelationCreations`) and the relation/entity-creation calls in `processEntityCreations`.
- **SIG — Tool allowlist not enforced:** Added `audit.ToolCLI` / `ToolMCP` / `ToolDataEntry` / `ToolScheduler` / `ToolDesktop` constants. Entry points reference constants, not string literals.
- **SIG — MCP principal semantics undefined:** Documented. MCP records the host-process user (the operator who launched `rela mcp ...`), not the LLM caller. Per-call principal-from-protocol is a future-MCP-variant concern, not in scope.
- **MINOR — Lua-eval inheritance implicit:** Documented as a known v1 limitation. Lua-eval writes carry `Tool: "cli"` in v1; future sub-tool field can disambiguate if forensic granularity demands it.
