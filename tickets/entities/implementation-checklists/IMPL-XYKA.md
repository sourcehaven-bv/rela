---
id: IMPL-XYKA
type: implementation-checklist
title: 'Implementation: Audit log: append-only JSONL of entity write operations'
status: pending
---

<!-- @managed: claude-workflow v1 -->

## Implementation handoff notes

Picking up from PLAN-XKMJ (status: done). All design decisions confirmed with
the user; no further plan revisions needed before coding. Outstanding choice
(resolved during planning): **strict nil-rejection** on workspace constructor —
production `New()` rejects missing audit. Tests get an ergonomic default via
`NewForTest` (see helper strategy below).

### Helper-first discipline

User feedback during planning: "if a lot of test refactoring is needed, that's a
sign we need helpers so future changes don't hit the same code again." This
applies here — adding a new required collaborator to `workspace.New` /
`WriteDeps` shouldn't churn 20+ call sites. Build the helpers **first**, then
thread the collaborator through.

**Helper changes to land before the audit wiring:**

1. **`workspace.NewForTest`** auto-populates `Audit` to `audit.Nop{}` if `WithAudit` is not supplied. AC10's "constructor rejects nil" still holds for production `New()`; `NewForTest` is the explicit "give me sensible test defaults" entry point and is allowed to pre-populate. Tests that assert on records use `WithAudit(audit.NewMemory())`.

2. **`internal/cli/root.go` already centralizes `workspace.Discover(...)` for CLI commands** (line 86). Make this the single place that constructs production audit (`audit.NewFilesystem(...)`) and passes it via `WithAudit(...)`. The 7 call sites in `internal/cli/*.go` that currently call `workspace.Discover` directly should be migrated to a shared helper if they aren't already. Validate this when implementing — there may already be a `cliWorkspace()` style helper.

3. **`cmd/rela-server/main.go` and `cmd/rela-desktop/main.go`** each construct workspace from scratch. Consider extracting `cmd/internal/bootstrap` (or similar) with a `BuildWorkspace(paths, audit, scriptExec)` helper. If both binaries plus `cli/root.go` use the same helper, future required collaborators land in one spot. (If the call sites are too divergent for a shared helper, document why.)

4. **`internal/dataentry/test_helpers_test.go`** — wrap workspace construction in a single test helper (e.g. `newTestApp(t, opts...)`). Currently spreads `workspace.NewForTest(meta, workspace.WithFS(...))` across multiple files. Test helper hides audit (`WithAudit(audit.Nop{})`) and any other collaborators, so adding collaborator #N+1 doesn't ripple.

After these helpers exist, threading audit through is small and additive.

### Suggested implementation order (revised)

1. **Build `internal/audit/` package first** (zero dependencies on the rest). Files:
   - `audit.go` — `Audit` interface, `Record` struct (JSON tags per plan).
   - `nop.go` — `Nop` type.
   - `memory.go` — `Memory` backend with mutex + records slice + `Records()` snapshot accessor.
   - `filesystem.go` — JSONL writer with daily UTC rotation, internal mutex, lazy file open. Constructor rejects empty dir/actor. `Record()` returns no error; logs `audit.write_failed` via `slog.Error` on failure (per AC8 / Decision 2).
   - `context.go` — `WithTriggeredBy(ctx, label)` / `TriggeredByFrom(ctx)` helpers using a private `triggeredByKey` type.
   - `actor.go` — `ResolveActor()` chain: `$RELA_ACTOR` → `$USER` → `git config user.email` → `"system"`. Length-cap, control-char strip.
   - Tests for each (cover all unit-test items from the plan).

2. **Wire `Audit` into `lua.WriteDeps`** (`internal/lua/deps.go`). One field add.

3. **Refactor test/production setup to use helpers** (helper-first discipline above). This is the step that pays the dividend on future collaborator additions.

4. **Update workspace** (`internal/workspace/`):
   - Add `WithAudit(audit.Audit) Option`.
   - `Workspace` struct: add `audit audit.Audit` field.
   - `New(...)` requires audit option to be set; returns error otherwise.
   - `NewForTest(...)` auto-defaults to `audit.Nop{}` if no `WithAudit` supplied (the helper carve-out).
   - `LuaWriteDeps()` / `LuaReadDeps()` — set `Audit` on the WriteDeps.
   - `wsEntityManager` (`manager.go`) — add `recordAudit(ctx, op, entityType, entityID, summary)` helper; invoke on each of the 7 write methods' success paths. Reads `audit.TriggeredByFrom(ctx)`.

5. **Automation engine plumbing** in `workspace.go`. Find the path where the workspace executes Lua actions emitted by automations (`scriptExec.ExecuteCode`/`ExecuteFile` calls in the automation cascade). Wrap the ctx passed to subsequent EntityManager calls with `audit.WithTriggeredBy(ctx, "automation:"+luaToExec.AutomationName)`. Same for direct (non-Lua) automation actions: when the workspace calls back into `createEntity` / `createRelation` / set-property on behalf of an automation, the surrounding ctx should carry the automation label.

Note: `workspace.LuaToExecute.AutomationName` already carries the right value;
we just need to thread it through the ctx where the workspace re-enters the
manager.

6. **Scheduler plumbing** (`internal/scheduler/scheduler.go:170+ doExecuteTask`). Derive `ctx := audit.WithTriggeredBy(parent, "schedule:"+task.Name)` before `engine.ExecuteFile(...)`.

7. **Wire entry points** (now small thanks to the helpers from step 3):
   - `cmd/rela-server/main.go` and `cmd/rela-desktop/main.go` — through the shared bootstrap helper.
   - `internal/cli/root.go` — through the shared cliWorkspace helper.
   - `internal/dataentry/app.go` (`luaWriteDeps`) — propagate from workspace.

8. **Tests** for the integration:
   - `internal/workspace/manager_audit_test.go` — table-driven over the 7 write methods, assert exactly one record per op (AC1, AC2).
   - Automation-cascade test (AC4).
   - Scheduler-driven test (AC5) — fixture schedule + Lua script.
   - Failing-backend test (AC8) — wrap Memory with a stub that returns errors; assert write succeeds + slog.Error captured.

9. **Manual e2e verification**:
   - `just dev`, perform a write via the data-entry UI, `cat .rela/audit/$(date -u +%Y-%m-%d).jsonl`.
   - Trigger a metamodel automation, confirm the cascade record carries `triggered_by`.
   - Run the scheduler with a fixture schedule, confirm `triggered_by: schedule:<name>`.

### Gotchas surfaced during planning

- **Workspace `wsEntityManager` ignores ctx today** (`_ context.Context` in every method). When you fix this to `ctx context.Context`, verify lint exemptions still apply.
- **Automation cascades ctx propagation**: the workspace re-enters its own manager from automation-execution paths. Make sure those inner calls receive a *child* ctx with `WithTriggeredBy` set, not a fresh `context.Background()`.
- **Nop-rejection AC10 in production**: don't silently substitute `Nop{}` in `New()`. Must `return nil, errors.New("workspace: audit is required (use WithAudit)")`.
- **`internal/cli/validate.go` uses `NopScriptExecutor`** because validation doesn't run scripts. It still needs `WithAudit(audit.Nop{})` (or routes through the shared cliWorkspace helper that supplies it). Validation does write nothing, so Nop is appropriate.
- **The helper extraction (step 3) is real code, not boilerplate**. It needs its own small tests and may surface existing duplicated setup logic worth consolidating.

## Development

- [ ] Unit tests written for new code
- [ ] Integration tests written (test full flow, not just units)
- [ ] Happy path implemented
- [ ] Edge cases from planning handled
- [ ] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [ ] Using fixture builders or factories for test data
- [ ] No hardcoded values in assertions when object is in scope
- [ ] Only specifying values that matter for the test
- [ ] Interpolated values constructed from objects, not hardcoded
- [ ] Property comparisons use original object, not hardcoded strings
- [ ] Test-setup helpers extracted so future collaborators don't ripple

## Manual Verification

- [ ] Feature manually tested end-to-end
- [ ] Each acceptance criterion verified with test scenario from planning
- [ ] Edge cases manually verified

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [ ] Code follows project patterns (check similar code)
- [ ] No security issues introduced
- [ ] No silent failures (errors logged AND returned)
- [ ] No debug code left behind
- [ ] Helpers extracted (step 3) so adding the next required collaborator doesn't churn 20 call sites again
