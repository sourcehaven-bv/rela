---
id: PLAN-K0RQ
type: planning-checklist
title: 'Planning: Delegate wsEntityManager to entitymanager.Manager (wire Manager into production)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `entitymanager.Manager` (TKT-QTNX) exists but is unused in
production. Every consumer reaches EntityManager via `ws.EntityManager()`,
which returns `wsEntityManager`, which delegates to workspace's own
`createEntity` / `updateEntity` / `deleteEntity` (~400 LOC of duplicate
automation+cascade orchestration). The new Manager sits alongside.

**Goal.** Flip the wiring so `wsEntityManager` forwards to a held
`*entitymanager.Manager`. The legacy workspace write methods become dead
code and get deleted. After this, every `ws.EntityManager()` call actually
exercises the new Manager.

**Scope (revised after design review):**

In scope:

1. **Port DEC-HWZHA soft-validation surface to entitymanager.Manager.**
   `Manager.CreateEntity` / `UpdateEntity` currently reject *any*
   validation error with `*ValidationError`. They must distinguish hard
   structural errors (unknown entity type, ID prefix mismatch) which
   abort the write, from soft conditions (required-field-missing,
   type mismatch, invalid enum, bad date format) which proceed and
   populate `Result.Warnings`. See finding #2 in the design review.
2. **Re-export sentinel errors so existing callers keep working.**
   `workspace.ErrHasRelations` becomes a public alias of
   `entitymanager.ErrHasRelations` (same identity, not just same string).
3. **Construct one `*entitymanager.Manager`** at the right point in
   `New()` / `NewForTest()` — AFTER the real store is assigned. The
   placeholder `memstore.New()` in `newWorkspace` must not leak into
   Manager.Deps.Store.
4. **Build `wsScriptRunner` adapter** that resolves `LuaWriteDeps` per
   call. Holds `*Workspace`. Lives in a new file
   `internal/workspace/wsscriptrunner.go`.
5. **Rewrite `wsEntityManager` methods** to forward to `w.manager`.
   Each method ≤10 lines.
6. **Delete `workspace.createEntity` / `updateEntity` / `deleteEntity` /
   `createEntityCore`** (~400 LOC including helpers).
7. **Delete `workspace.createRelation` / `updateRelation` /
   `deleteRelation`** (~150 LOC). They have no automation/cascade
   surface but they do have validation + template-application logic
   that's already in Manager. wsEntityManager forwards directly to
   Manager for relations too.
8. **Delete `internal/workspace/autocascade_host.go`** (~146 LOC).
   Manager has its own cascadeHost.
9. **Drop the `runner *autocascade.Runner` field** from Workspace and
   the `newWorkspace` lines 304-327 that build it.
10. **Drop `entityNotFoundError`** local type in workspace/manager.go —
    Manager produces `ErrEntityNotFound`-wrapped errors that callers
    can `errors.Is`.
11. **Keep** `lookupEntity`, `writeEntity` (`SeedEntityForTest`),
    `writeRelation` (`SeedRelationForTest`) — they remain test fixtures
    that operate on the store directly without going through Manager.
12. **Migrate test callsites** that call `ws.createEntity(...)` etc. to
    `ws.EntityManager().CreateEntity(ctx, ...)`. ~44 occurrences across
    `workspace_test.go`, `rename_test.go`, `validation_softening_test.go`,
    `query_test.go`. Build a small `mustCreate(t, ws, type, props)`
    helper to absorb the entity-construction boilerplate.
13. **Update `cli/delete.go:75`** from
    `errors.Is(err, workspace.ErrHasRelations)` to the same call —
    only works because (2) re-exports the sentinel as an alias.
14. **arch-lint update**: drop `automation` from
    `workspace.mayDependOn` (Manager owns it). Keep `autocascade`
    (wsScriptRunner imports `autocascade.ScriptRunner`,
    `autocascade.ScriptAction`). Keep `entitymanager` (workspace
    constructs Manager). Keep `lua` (LuaWriteDeps).
15. **Delete `TestRename_ErrorTypeMismatch`** in
    `internal/workspace/rename_test.go:364-375`. The entity-type
    cross-check is no longer surfaced via any public API. Document the
    feature removal in the deletion commit.

Out of scope:

- Removing `wsEntityManager` adapter (TKT-64R3 does that with Workspace).
- Per-command migration (TKT-KWAX / 0SP1 / 9JEI / 2IAC).
- Audit / principal / policy hooks.
- Changing the `EntityManager` interface shape.
- `EntityManager()` allocator memoisation (leverage item, separate ticket).

**Acceptance Criteria:**

1. `internal/workspace.Workspace` holds `*entitymanager.Manager`,
   constructed in `New()` and `NewForTest()` AFTER `ws.store` is
   assigned to the real store (not the placeholder memstore from
   `newWorkspace`). Verify: read both constructors; grep
   `entitymanager.New` and confirm only those two callers in workspace.
2. `wsEntityManager.CreateEntity / UpdateEntity / DeleteEntity /
   RenameEntity / CreateRelation / UpdateRelation / DeleteRelation`
   bodies are ≤10 lines each, just field translation + forward.
3. `workspace.createEntity`, `workspace.updateEntity`,
   `workspace.deleteEntity`, `workspace.createEntityCore`,
   `workspace.createRelation`, `workspace.updateRelation`,
   `workspace.deleteRelation` no longer exist. Verify with grep.
4. `internal/workspace/autocascade_host.go` does not exist.
5. `Workspace.runner` field gone; `newWorkspace` does not construct an
   `autocascade.Runner`. Verify: grep `runner.*autocascade.Runner`
   returns no matches in `internal/workspace/`.
6. `workspace.ErrHasRelations` is `entitymanager.ErrHasRelations`
   (alias, same identity). Existing callers in `cli/delete.go:75` and
   `workspace_test.go:566` work unchanged. Verify:
   `errors.Is(workspace.ErrHasRelations, entitymanager.ErrHasRelations) == true`.
7. `entitymanager.Manager.CreateEntity` and `UpdateEntity` partition
   validation errors into hard (abort) vs soft (write+warn) per
   DEC-HWZHA. `internal/workspace/validation_softening_test.go` and
   `TestCreateEntity_RequiredMissingSurfacesWarning` pass.
8. `entitymanager.partitionValidationErrors` exists with the same
   behavior as the current workspace one; the workspace helper either
   delegates to it or is deleted.
9. Manager's `CreateResult.Warnings` and `UpdateResult.Warnings` get
   populated with soft conditions on every write.
10. All existing workspace tests pass.
11. `just ci` is green.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing solutions / prior art:**

- **`entitymanager.Manager`** (TKT-QTNX, PR #702) — already exists with
  full pipeline. This ticket is plumbing, not algorithm work, with one
  exception: the DEC-HWZHA validation softening needs to be ported from
  workspace into entitymanager (see Approach #1 below).
- **`internal/workspace/errors.go:35-65` `partitionValidationErrors`** —
  the function we're porting. Pure metamodel logic; trivial to move.
- **`internal/workspace/workspace.go:865-869, 901-905, 925-929`** — the
  three workspace use sites that show how soft vs hard errors flow.
- **`internal/workspace/manager.go`** (wsEntityManager, 171 LOC) — the
  shape we're shrinking. After the flip it becomes a ≤80 LOC
  forwarder.
- **`internal/workspace/workspace.go:800-1100`** —
  `createEntity` / `updateEntity` / `deleteEntity` / `createEntityCore`
  plus the cascade-dispatch blocks. Roughly 300 LOC.
- **`internal/workspace/workspace.go:1138-1265`** — relation methods
  (`createRelation` / `updateRelation` / `deleteRelation` /
  `writeRelationCore`). ~150 LOC.
- **`internal/workspace/autocascade_host.go`** — 7 methods, 146 LOC.
  Whole file deleted.
- **`internal/workspace/services.go:38` `LuaWriteDeps()`** — captures
  Store/Tracer/Searcher/Meta/projectRoot/EntityManager. All stable
  values; the wsScriptRunner adapter resolves it per call to match
  current behavior exactly.
- **`internal/workspace/luascriptrunner.go`** — existing per-call
  adapter that workspace.createEntity instantiates today via
  `newLuaScriptRunner(w.scriptExec, w.LuaWriteDeps())`.
  wsScriptRunner just wraps the same construction in a stable closure.

**Reference implementations:**

- Architect/cranky reviews on TKT-QTNX repeatedly argued for plain
  field ScriptRunner over a factory. That decision is correct for
  consumers that hold stable deps. wsScriptRunner is the call-site
  adapter pattern those reviews described — per-call resolution lives
  at the wiring site, not on Manager.Deps.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Implementation steps in dependency order:**

### Step 1 — Port DEC-HWZHA validation softening to entitymanager.Manager

This is the precursor work that TKT-QTNX shipped without. It must land
first because every subsequent step delegates to Manager.

Move `partitionValidationErrors` from
`internal/workspace/errors.go:35-65` into
`internal/entitymanager/validation.go` (new file). It depends only on
`metamodel.ValidationError`, which entitymanager already imports.

Update Manager:

```go
// internal/entitymanager/core.go - in createCore
if errs := deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties); len(errs) > 0 {
    hardErrs, soft := partitionValidationErrors(errs)
    if len(hardErrs) > 0 {
        return nil, newValidationError(hardErrs)
    }
    // soft conditions ride along; written to Result.Warnings by caller
    e.softWarnings = soft  // or return alongside *entity.Entity
}
```

The caller (Manager.CreateEntity / UpdateEntity) populates
`result.Warnings` from the soft slice. Two options for plumbing it
through `createCore`:

- **(a) Return `(*entity.Entity, []Warning, error)`** from `createCore`.
  Clean, explicit; touches every cascadeHost call site (one — see
  `internal/entitymanager/cascadehost.go:37`).
- **(b) Compute warnings in CreateEntity/UpdateEntity** by re-running
  `ValidateEntity` on the post-write entity (mirrors workspace's
  current shape at workspace.go:867-869). One extra ValidateEntity
  call per write — cheap.

Pick **(b)** to minimise the API surface change in createCore. UpdateEntity
already calls ValidateEntity once at the top; collapse to one call
that partitions and stashes both buckets, then fail on hard, persist
on soft.

Also update workspace.go to stop computing warnings itself once
wsEntityManager forwards (workspace's three sites become dead code in
later steps).

### Step 2 — Re-export sentinels for backward compat

In `internal/workspace/workspace.go` (or a new `internal/workspace/errors.go`
section):

```go
import "github.com/Sourcehaven-BV/rela/internal/entitymanager"

// ErrHasRelations is an alias for entitymanager.ErrHasRelations so
// existing callers (cli/delete.go, workspace_test.go) keep working.
// Deleted along with the workspace package in TKT-64R3.
var ErrHasRelations = entitymanager.ErrHasRelations
```

Same identity, same string, same errors.Is behavior. No call-site
changes needed.

### Step 3 — Build wsScriptRunner adapter

New file `internal/workspace/wsscriptrunner.go`:

```go
package workspace

import (
    "context"
    "github.com/Sourcehaven-BV/rela/internal/autocascade"
)

// wsScriptRunner is the per-call autocascade.ScriptRunner adapter the
// workspace constructs once at New() time. It resolves lua.WriteDeps
// per dispatch (same as today's pattern in workspace.createEntity).
//
// The per-call resolution preserves correctness under workspace reload:
// LuaWriteDeps captures pointer-typed fields (Store, Tracer, ...) that
// may be replaced on reload; capturing the bundle once at construction
// would freeze them.
type wsScriptRunner struct{ w *Workspace }

func (r *wsScriptRunner) Run(ctx context.Context, a autocascade.ScriptAction) error {
    return newLuaScriptRunner(r.w.scriptExec, r.w.LuaWriteDeps()).Run(ctx, a)
}
```

Note `r.w.LuaWriteDeps().EntityManager` returns `w.EntityManager()` which
after the flip is the wsEntityManager forwarder to `w.manager`. The
chicken/egg is resolved because Manager is the *target* of the
forwarder, not part of LuaWriteDeps's construction graph.

### Step 4 — Construct Manager in New() and NewForTest()

**Important: NOT in newWorkspace.** `newWorkspace` initializes
`store: memstore.New()` at line 359 as a placeholder; the real store
gets assigned in `New()` at line 189 and `NewForTest()` at line 260.
Building Manager in newWorkspace would bind it to the throwaway memstore.

After each store assignment, add:

```go
// internal/workspace/workspace.go, in New() after `ws.store = s`:
mgr, err := entitymanager.New(entitymanager.Deps{
    Store:        ws.store,
    Meta:         ws.meta,
    Templater:    ws.Templater(),
    Automations:  ws.automation,
    Cascade:      ws.runner,         // build once more here; deleted in step 8
    ScriptRunner: &wsScriptRunner{w: ws},
})
if err != nil {
    return nil, fmt.Errorf("build entitymanager: %w", err)
}
ws.manager = mgr
```

Same block (sans error wrap) in NewForTest. Construct Cascade INSIDE
this block (move it down from newWorkspace lines 304-327). Then drop
the now-unused runner field from Workspace and the corresponding
newWorkspace block — that's step 8.

### Step 5 — Rewrite wsEntityManager forwarders

Replace `internal/workspace/manager.go` body:

```go
func (m *wsEntityManager) CreateEntity(ctx context.Context, e *entity.Entity, opts entitymanager.CreateOptions) (*entitymanager.CreateResult, error) {
    return m.w.manager.CreateEntity(ctx, e, opts)
}

func (m *wsEntityManager) UpdateEntity(ctx context.Context, e *entity.Entity) (*entitymanager.UpdateResult, error) {
    return m.w.manager.UpdateEntity(ctx, e)
}

func (m *wsEntityManager) DeleteEntity(ctx context.Context, id string, cascade bool) (*entitymanager.DeleteResult, error) {
    return m.w.manager.DeleteEntity(ctx, id, cascade)
}

func (m *wsEntityManager) RenameEntity(ctx context.Context, oldID, newID string, opts entitymanager.RenameOptions) (*entitymanager.RenameResult, error) {
    return m.w.manager.RenameEntity(ctx, oldID, newID, opts)
}

func (m *wsEntityManager) CreateRelation(ctx context.Context, from, relType, to string, opts entitymanager.RelationOptions) (*entity.Relation, error) {
    return m.w.manager.CreateRelation(ctx, from, relType, to, opts)
}

func (m *wsEntityManager) UpdateRelation(ctx context.Context, from, relType, to string, opts entitymanager.RelationOptions) (*entity.Relation, error) {
    return m.w.manager.UpdateRelation(ctx, from, relType, to, opts)
}

func (m *wsEntityManager) DeleteRelation(ctx context.Context, from, relType, to string) error {
    return m.w.manager.DeleteRelation(ctx, from, relType, to)
}
```

Drop `_ = opts.Variant` workaround (Variant now plumbs through; no
production caller passes it — verified by grep, see Risk #5).
Drop `current.Clone()` + Properties wholesale-replace in UpdateEntity
(Manager assumes the caller hands it the complete intended state; all
callers today do `GetEntity` first, then mutate, then call UpdateEntity).
Drop entity-type lookup before RenameEntity (Manager doesn't need it).
Delete `entityNotFoundError` local type (no callers after the flip).

### Step 6 — Delete the dead workspace methods

In one commit:

- `workspace.createEntity` (workspace.go:800-872)
- `workspace.updateEntity` (workspace.go:896-952)
- `workspace.deleteEntity` (workspace.go:964-1002)
- `workspace.createEntityCore` (workspace.go:1014-1100)
- `workspace.createRelation` (workspace.go:1138-1206)
- `workspace.updateRelation` (workspace.go:1208-1233)
- `workspace.deleteRelation` (workspace.go:1235-1265)
- `workspace.writeRelationCore` (workspace.go:1074+) — only called by
  the methods above
- `workspace.deleteEntityStore` (workspace.go:427-439) — only called
  by workspace.deleteEntity and autocascade_host.go (both deleted)
- `workspace.deleteRelationStore` (workspace.go:450-460) — same
- Local types: `CreateOptions`, `CreateRelationOptions`, `CreateResult`,
  `UpdateResult`, `DeleteResult`, `createEntityCoreOpts`. Keep only if
  still referenced externally (some tests; check).

KEEP:

- `workspace.writeEntity` (workspace.go:417-422) — used by
  `SeedEntityForTest`.
- `workspace.writeRelation` (workspace.go:442-447) — used by
  `SeedRelationForTest`.
- `workspace.lookupEntity` (query.go:20) — test helper.
- `workspace.rename` (rename.go:16-21) — already a thin shim over
  `rename.Rename`; no harm leaving it.

### Step 7 — Delete autocascade_host.go entirely

`internal/workspace/autocascade_host.go` (~146 LOC) — Manager has its
own cascadeHost. No external references after step 6.

### Step 8 — Drop the runner field and its construction

In `internal/workspace/workspace.go`:

- Remove `runner *autocascade.Runner` from the `Workspace` struct.
- Remove lines 304-327 in `newWorkspace` that build it.
- Construction moves into the Manager block in New()/NewForTest() per
  step 4.

### Step 9 — Update arch-lint

Predicted diff for `.go-arch-lint.yml` workspace.mayDependOn:

- **Drop**: `automation` (workspace no longer imports it directly;
  Manager.Deps.Automations is built and passed but workspace types
  it as `*automation.Engine` via entitymanager re-export… actually
  let me verify). Run `goimports` after the deletes and re-check.

After running goimports on workspace files post-deletion:

- Workspace imports `entitymanager` (for `Manager`, `Deps`).
- Workspace imports `autocascade` (for `Runner` construction at the
  Manager-build site, plus `ScriptRunner` for wsScriptRunner).
- Workspace imports `automation` (for `Engine` construction at the
  Manager-build site).
- Workspace imports `lua` (LuaWriteDeps/LuaReadDeps).

**Net change in arch-lint**: no removals expected; `entitymanager` is
already in `mayDependOn`. Verify via `just arch-lint` after the cuts.

If a dep DOES become unused (e.g., `validation`, `validator` because
they were only referenced via the deleted createEntityCore validation
path), trim it and document.

### Step 10 — Migrate test callsites

44 occurrences in workspace_test.go. Pattern:

```go
// Before:
entity, _, err := ws.createEntity("requirement", CreateOptions{
    Properties: map[string]interface{}{"title": "Test"},
})

// After:
e := &entity.Entity{Type: "requirement", Properties: map[string]interface{}{"title": "Test"}}
result, err := ws.EntityManager().CreateEntity(ctx, e, entitymanager.CreateOptions{})
entity := result.Entity
```

Build a `mustCreate` helper in the workspace test package:

```go
// internal/workspace/testhelpers_test.go
func mustCreate(t *testing.T, ws *Workspace, entityType string, props map[string]interface{}) *entity.Entity {
    t.Helper()
    e := &entity.Entity{Type: entityType, Properties: props}
    result, err := ws.EntityManager().CreateEntity(context.Background(), e, entitymanager.CreateOptions{})
    if err != nil { t.Fatalf("mustCreate(%s): %v", entityType, err) }
    return result.Entity
}
```

Drop `TestRename_ErrorTypeMismatch` (rename_test.go:364-375). The
entity-type cross-check is no longer surfaced via any public API.
This is a deliberate feature removal; document in the commit message.

### Step 11 — Update error-message string-matching tests to errors.Is

Workspace tests doing `strings.Contains(err.Error(), "entity not found")`
or similar must flip to `errors.Is(err, entitymanager.ErrEntityNotFound)`.
Grep for these:

- `strings.Contains(err.Error(),` in workspace test files.
- `err.Error() ==` in workspace test files.

Estimate: ~10-15 places, scattered across rename_test.go and
workspace_test.go.

**Files to modify:**

- `internal/entitymanager/validation.go` — NEW: ports
  `partitionValidationErrors`.
- `internal/entitymanager/manager.go` — partitions validation errors
  in CreateEntity/UpdateEntity; populates Result.Warnings.
- `internal/entitymanager/core.go` — same partitioning in createCore.
- `internal/entitymanager/manager_test.go` — add tests covering soft
  validation behavior (CreateResult.Warnings populated; write succeeds
  on soft-only errors).
- `internal/workspace/wsscriptrunner.go` — NEW: per-call adapter.
- `internal/workspace/manager.go` — wsEntityManager forwarders.
- `internal/workspace/workspace.go` — delete write methods, drop runner
  field, construct Manager in New()/NewForTest().
- `internal/workspace/autocascade_host.go` — **DELETE**.
- `internal/workspace/errors.go` — re-export ErrHasRelations; remove
  partitionValidationErrors (it moved to entitymanager).
- `internal/workspace/workspace_test.go` — migrate ~44 callsites.
- `internal/workspace/rename_test.go` — migrate callsites, delete
  TestRename_ErrorTypeMismatch.
- `internal/workspace/validation_softening_test.go` — should pass
  unchanged after Manager gains DEC-HWZHA support; verify.
- `internal/workspace/query_test.go` — migrate 2 callsites.
- `.go-arch-lint.yml` — verify workspace.mayDependOn still correct
  after deletions.

**Alternatives considered:**

- **Capture LuaWriteDeps once at Manager construction.** Rejected:
  fields are pointer-typed and workspace reload may replace them.
  Per-call resolution via wsScriptRunner matches today exactly.
- **Build Manager in `newWorkspace` and re-build in New() after store
  assignment.** Rejected: doubles the wiring, easy to forget to
  rebuild, no benefit.
- **Memoise `EntityManager()` with sync.Once.** Currently allocates
  `&wsEntityManager{w: w}` per call. Cheap; not in scope for this
  ticket. Filed as a leverage opportunity.
- **Add `EntityType` field to `entitymanager.RenameOptions`** to
  preserve workspace.rename's type cross-check. Rejected: no
  production caller depends on the check; CLI's `rename` command
  uses lookup-by-ID before calling RenameEntity and trusts the
  loaded type.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:** None new. This ticket reroutes
existing write paths; all inputs continue through the same metamodel
validation (now in Manager). DEC-HWZHA soft-vs-hard partitioning is
ported verbatim from workspace.

**Security-sensitive operations:** None new. Lua script execution
stays per-call (wsScriptRunner resolves LuaWriteDeps fresh per
cascade), so existing script-traversal protections in
`lua.WriteDeps.Run` remain in effect.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test scenarios per AC:**

- AC1 (Manager constructed in New/NewForTest after store): every
  workspace constructor test (`TestNew`, `setupTestWorkspace`, etc.)
  exercises this. Failure mode: nil-store error from `entitymanager.New`.
- AC2 (wsEntityManager methods small): visual review during PR.
- AC3 (workspace write methods deleted): grep target; PR includes
  the grep run in the commit message.
- AC4-AC5: file-level grep verification.
- AC6 (ErrHasRelations identity): explicit test
  `TestErrHasRelations_Alias` that asserts
  `errors.Is(workspace.ErrHasRelations, entitymanager.ErrHasRelations)`.
- AC7 (DEC-HWZHA in Manager): pinned by re-running
  `internal/workspace/validation_softening_test.go` against the
  forwarded path; should pass without modification.
- AC8 (partitionValidationErrors moved): grep; no callers in workspace
  after step 1.
- AC9 (Warnings populated): new test
  `TestCreate_SoftValidationProducesWarning` in
  `internal/entitymanager/manager_test.go` — required-missing entity
  succeeds with one Warning entry; hard error (unknown type) returns
  *ValidationError.

**Edge cases preserved from current workspace behavior:**

- Cascade-driven entity creation skips automation (no recursion).
  Already pinned by `TestRunnerDepthLimit` in autocascade.
- `customIDNotAllowedError` wording. Manager has its own typed error;
  workspace tests asserting on the message format may need flipping
  to `errors.As(err, &entitymanager.ValidationError{})` or matching
  the new wording. Audit.
- Lua-driven recursion: cascade fires inside Lua → reaches
  `wsEntityManager.CreateEntity` → Manager.CreateEntity → another
  cascade. Bounded by `autocascade.MaxDepth`. Pinned by
  `TestLuaAutomation_*` in workspace_test.go.

**Negative tests:**

- Workspace constructor returns error when Manager construction fails
  (covered indirectly — newWorkspace always supplies non-nil store/
  meta/templater).
- Soft validation conditions don't abort the write. New test in
  entitymanager (AC9).

**Integration test approach:** every existing workspace test becomes
an integration test of `wsEntityManager → Manager → store`. No
test-shape change beyond the call-site migration.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**

1. **DEC-HWZHA port surface.** The hardest part of the ticket.
   Partitioning needs to land in Manager (createCore, CreateEntity,
   UpdateEntity) and Result.Warnings needs to flow through. New tests
   required. Mitigation: do this first (step 1), with its own test
   coverage, before touching workspace.

2. **Construction-order bug.** Building Manager inside `newWorkspace`
   captures the placeholder memstore (line 359). Production writes
   would silently bypass fsstore. Mitigation: construct Manager in
   `New()` and `NewForTest()` AFTER store assignment. Code review
   check: only two `entitymanager.New(` callers in internal/workspace.

3. **ErrHasRelations identity drift.** `cli/delete.go:75` does
   `errors.Is(err, workspace.ErrHasRelations)`. Manager returns
   `entitymanager.ErrHasRelations` (different identity, same string).
   Mitigation: re-export `workspace.ErrHasRelations =
   entitymanager.ErrHasRelations` for the duration of the workspace
   shim. Acceptance test pins this (AC6).

4. **wsScriptRunner construction order.** wsScriptRunner holds `*Workspace`
   and resolves `w.LuaWriteDeps()` per call. Per call, `LuaWriteDeps()`
   builds a fresh `wsEntityManager{w: w}`. By the time the first cascade
   fires, `w.manager` is set (Manager is constructed BEFORE any write
   path can be invoked because New() returns the workspace before any
   caller can mutate). Pinned by `TestLuaAutomation_*` re-running
   unchanged.

5. **Variant suddenly working.** Today wsEntityManager drops
   `opts.Variant`. **Verified safe**: grep confirms zero production
   callers pass Variant (`cli/create.go:98`, `mcp/tools_entity.go:169`,
   `lua/runtime.go:1296`, `dataentry/handlers_api.go:410`,
   `dataentry/api_v1.go:449,963` — none set `Variant`).
   `cli/template.go` uses a different code path. After the flip,
   Variant becomes functional but unused.

6. **Test callsite migration.** ~44 workspace test callsites + ~15
   error-message-matching sites. `mustCreate` helper absorbs the
   bulk. Mitigation: do this in one commit with a clear before/after
   pattern so review is mechanical.

7. **TestRename_ErrorTypeMismatch removal.** Pins a feature
   (entityType cross-check at rename time) that no API caller
   depends on. Mitigation: explicit deletion with rationale in commit
   message. If anyone asks, the feature can be re-added via
   `entitymanager.RenameOptions.EntityType` in a follow-up.

8. **arch-lint footprint changes.** Predicted no removals from
   `workspace.mayDependOn` because workspace still imports
   `entitymanager`, `autocascade`, `automation`, `lua` for the
   Manager build site and wsScriptRunner. Verified by reading the
   import statements after dead-code deletion. Mitigation: run
   `just arch-lint` after deletions; restore any dep that fails.

**Effort:** **L** (large). Revised estimate: 4-6 hours.

- Step 1 (DEC-HWZHA port + tests): 1.5 h
- Steps 2-9 (the flip + deletions): 1.5 h
- Step 10 (test callsite migration): 1.5 h
- Step 11 (error-message migration): 0.5 h
- Review iterations + arch-lint cleanup: 0.5-1 h

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation impact:** N/A — Internal refactor with intentional
preservation of all observable semantics (pipeline order, error types,
soft-validation behavior, cascade depth limit, automation-set property
second-write).

One feature removal documented in the commit message:
`TestRename_ErrorTypeMismatch` and the `EntityType` field in
`rename.Options` go away. No production caller depends on it.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings (round 1, cranky-code-reviewer):**

14 findings (3 critical, 7 significant, 4 minor/nit). All addressed in
this revised plan:

- **CRITICAL #1** (construction order memstore-vs-fsstore): addressed
  by Step 4 + AC1.
- **CRITICAL #2** (DEC-HWZHA validation regression): addressed by
  Step 1 + AC7/8/9.
- **CRITICAL #3** (ErrHasRelations identity drift): addressed by
  Step 2 + AC6.
- **CRITICAL #4** (wsScriptRunner termination/back-reference): walked
  through in Step 3 commentary + Risk #4. Pinned by existing
  `TestLuaAutomation_*` tests.
- **SIGNIFICANT #5** (TestRename_ErrorTypeMismatch): addressed by
  scope item #15 + Risk #7.
- **SIGNIFICANT #6** (test callsite migration shape): addressed by
  Step 10 + `mustCreate` helper.
- **SIGNIFICANT #7** (relation methods + lookupEntity + rename fate):
  addressed by scope items #11 (keep) and #6/#7 (delete relation
  methods).
- **SIGNIFICANT #8** (UpdateEntity Clone+overwrite dance): addressed
  by Step 5 commentary; callers GetEntity-then-mutate today, no
  behavior change.
- **SIGNIFICANT #9** (entityNotFoundError deletion): addressed by
  scope item #10.
- **SIGNIFICANT #10** (arch-lint diff prediction): addressed by
  Step 9 with explicit prediction.
- **SIGNIFICANT #11** (Variant: positive assertion): addressed by
  Risk #5 with grep evidence.
- **MINOR #12** (cleanup conditional list): addressed by Step 6
  enumeration.
- **MINOR #13** (Approach #1 indecisive prose): addressed by
  rewriting Step 3 as linear sequence.
- **MINOR #14** (effort estimate): revised to L / 4-6 hours.
