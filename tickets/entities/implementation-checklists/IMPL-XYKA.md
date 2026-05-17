---
id: IMPL-XYKA
type: implementation-checklist
title: 'Implementation: Audit log: append-only JSONL of entity write operations'
status: pending
---

<!-- @managed: claude-workflow v1 -->

## Implementation handoff notes

Picking up from PLAN-XKMJ (status: done, rewritten 2026-05-17 against the
post-workspace-decomposition codebase). All design decisions confirmed; the plan
revision reflects the current package layout.

### Key shift from the original handoff

The original handoff worried about helper extraction across many workspace call
sites. The workspace-decomposition arc (TKT-QTNX → IU2S → DS43 → UG3C → 64R3 /
2IAC) already solved that problem: `Manager` is constructed in exactly two
places — `appbuild.Discover` (production) and `appbuild.NewForTest` (tests).
Adding a new required `Audit` collaborator now ripples through **zero** test
files if `NewForTest` defaults to `audit.Nop{}` when no `WithTestAudit` is
supplied.

### Suggested implementation order

1. **Build `internal/audit/` package first** (zero dependencies on the rest). Files:
   - `audit.go` — `Audit` interface, `Record` struct (JSON tags per plan).
   - `nop.go` — `Nop` type.
   - `memory.go` — `Memory` backend with mutex + records slice + `Records()` snapshot accessor.
   - `filesystem.go` — JSONL writer with daily UTC rotation, internal mutex, lazy file open. Constructor rejects empty dir/actor. `Record()` returns no error; logs `audit.write_failed` via `slog.Error` on failure (per AC8 / Decision 2).
   - `context.go` — `WithTriggeredBy(ctx, label)` / `TriggeredByFrom(ctx)` helpers using a private `triggeredByKey` type.
   - `actor.go` — `ResolveActor()` chain: `$RELA_ACTOR` → `$USER` → `git config user.email` → `"system"`. Length-cap, control-char strip.
   - Tests for each (cover all unit-test items from the plan).

2. **Add `Audit` to `entitymanager.Deps`** and validate in `New`:
   - `internal/entitymanager/manager.go` — append `Audit audit.Audit` to `Deps`; add `if d.Audit == nil { return nil, errors.New("entitymanager: New: Audit is required") }`.
   - Compile will break at `appbuild.New` and `appbuild.NewForTest`. Fix those next.

3. **Wire `appbuild`:**
   - `internal/appbuild/appbuild.go` — `Discover` constructs `audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"), audit.ResolveActor())` and threads through. `New` (low-level constructor) validates required Audit, same error shape as entitymanager.
   - `internal/appbuild/testfixture.go` — add `WithTestAudit(audit.Audit) TestOption`; in the Manager-construction helper, default `auditImpl := audit.Nop{}` then override if WithTestAudit was supplied. This is the single carve-out that prevents test-file churn.

4. **Add `recordAudit` helper to `Manager`:**
   - `internal/entitymanager/manager.go` — small private helper that reads `audit.TriggeredByFrom(ctx)`, stamps `Time: time.Now().UTC()`, calls `m.deps.Audit.Record(...)`.
   - Invoke from the 7 write methods on their tail-success branch: CreateEntity, UpdateEntity, DeleteEntity, RenameEntity, CreateRelation, UpdateRelation, DeleteRelation.

5. **Autocascade plumbing** (`internal/autocascade/runner.go` or its mutator-adapter):
   - When the cascade re-enters the `Mutator` for a scripted/non-scripted automation action, derive `ctx := audit.WithTriggeredBy(parent, "automation:"+automationName)` first.
   - The automation name is already in scope in the cascade's action-execution path (verify exact field name during implementation — internals shifted with TKT-6OMC).

6. **Scheduler plumbing** (`internal/scheduler/scheduler.go`):
   - Locate the script-engine invocation per task.
   - Derive `ctx := audit.WithTriggeredBy(parent, "schedule:"+task.Name)` immediately before invoking.

7. **Integration tests:**
   - `internal/entitymanager/manager_audit_test.go` — table-driven over the 7 write methods, assert exactly one record per op using `appbuild.NewForTest(meta, appbuild.WithTestAudit(audit.NewMemory()))` (AC1, AC2).
   - Automation-cascade test in entitymanager (AC4).
   - Scheduler-driven test in `internal/scheduler/` (AC5) — fixture schedule + Lua script.
   - Failing-backend test (AC8) — wrap Memory with a stub that triggers slog.Error; assert write succeeds + slog warning captured.

8. **Manual e2e verification:**
   - `just dev`, perform a write via the data-entry UI, `cat .rela/audit/$(date -u +%Y-%m-%d).jsonl`.
   - Trigger a metamodel automation, confirm the cascade record carries `triggered_by: automation:<name>`.
   - Run the scheduler with a fixture schedule, confirm `triggered_by: schedule:<name>`.

### Gotchas (current codebase)

- **No `lua.WriteDeps` change needed.** Original plan added `Audit` to `WriteDeps`; the refactor that narrowed `WriteDeps.EntityManager` to `lua.Mutator` made that unnecessary — Lua-driven writes go through `Manager` which audits itself.
- **`entitymanager.New` already validates required collaborators** (Store, Meta, Templater plus the Automations/Cascade pair-check). Adding the Audit check follows the same pattern.
- **Automation cascade re-entry**: confirm the cascade calls back through the *Mutator interface* on the Manager (not a separate path). If there's a path that bypasses the Manager, that path needs ctx wrapping too — or it won't be audited.
- **`appbuild.NewForTest` panics on construction errors** (it's a test helper, panics are appropriate). Make sure the default-audit branch is set *before* the `entitymanager.New` call so the nil-check doesn't trip.

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
- [ ] `appbuild.NewForTest` default-audit carve-out keeps audit invisible to tests that don't care

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
