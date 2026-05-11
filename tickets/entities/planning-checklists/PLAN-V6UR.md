---
id: PLAN-V6UR
type: planning-checklist
title: 'Planning: Extract automation.Runner with consumer-side Host interface'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** the automation-result dispatch logic — translating
`automation.Result` into actual store writes, recursive automation invocation,
Lua script execution — lives inside `internal/workspace/workspace.go` as a
cluster of private methods (lines 1049–1310). Workspace is being decomposed
(CLAUDE.md flags it as a transitional shim). Lifting this dispatch into its own
service is the foundational step: every subsequent migration (EntityManager,
MCP, scheduler, dataentry, CLI) needs the service to exist before it can move
off Workspace.

The refactor applies the consumer-side-interface pattern documented in CLAUDE.md
under "Consumer-side interfaces for callbacks and cycles." The new package
declares a `Host` interface naming exactly the methods it calls back into;
Workspace implements `Host` during the transition; future
`entitymanager.Manager` will implement it later.

### Package home: `internal/autocascade`

The service lands in a new top-level package `internal/autocascade`. *Not*
inside `internal/automation`. The naming captures both concepts in one breath:
"automation cascade" — `autocascade.Runner` is the runner for automation
cascades.

**Why top-level and not `internal/automation/cascade`:**

- The project direction is small services with explicit dependencies (CLAUDE.md "Don't add a cross-subsystem service locator"; the workspace decomposition tickets implementing FEAT-workspace). A sub-package would let the heavy-dependency leaf import its parent silently and grow under cover of "this is part of automation." Top-level forces every dependency on `automation` to be a declared `mayDependOn`, same as every other service.
- `internal/automation` stays as it is today — pure rule evaluator, decides what should happen, no I/O, no side effects. `mayDependOn: [filter, metamodel]` unchanged. The package's identity (decider, not executor) is preserved.
- `internal/autocascade` is the executor. Holds `Runner`, `Host`, `Request`, `Outcome`. It imports `automation` (Engine, Result, EntityToCreate, LuaToExecute), plus the dependencies it needs to perform the side effects (store, templating, script, lua).
- Sub-package precedent (`internal/store/fsstore` etc.) is for "alternative implementations of the same concept." Cascade is not an alternative implementation of automation — it's a different responsibility.

**In scope:**

- New `internal/autocascade/` package (multiple files; see "Files to modify"):
  - `Host` interface — a single consumer-side interface declaring all entity/relation/template callbacks `Runner` needs from its caller. 9 methods; see "Host signature" below.
  - `Request` struct grouping per-call invocation data (trigger entity, old entity, automation result, lua deps).
  - `Outcome` struct with the cumulative effects after a cascade completes (created entities, created relations, warnings, errors).
  - `Runner` struct with `Engine` and `Scripts` as fields. Constructor `New(Deps{...}) (*Runner, error)` rejects nil required fields (matches the pattern TKT-QTNX will use for `entitymanager.New`).
  - `Runner.Process(ctx context.Context, host Host, req Request) (Outcome, error)` — the BFS queue loop currently in `Workspace.applyAutomationSideEffects`. Host passed per-call to dissolve the future cycle with EntityManager.
  - `MaxDepth = 50` exported constant (so callers can verify the limit).
- Move these private Workspace methods into `autocascade.Runner`, parameterized over Host:
  - `applyAutomationSideEffects` (1049–1089) → `Runner.Process`
  - `processEntityCreations` (1092–1130) → `Runner.processEntityCreations`
  - `runCreatedEntityAutomation` (1133–1167) → `Runner.runCreatedEntityAutomation`
  - `applyRelationCreations` (1170–1199) → `Runner.applyRelationCreations`
  - `executeLuaActions` (1202–1245) → `Runner.executeLuaActions`
  - `handleIfExists` (1270–1307) → `Runner.handleIfExists`
  - `createTriggerRelation` (1310–1334) → `Runner.createTriggerRelation`
- `maxAutomationDepth = 50` moves into the `autocascade` package as `MaxDepth`.
- `ScriptExecutor` interface moves from `internal/workspace` to `internal/script` (its natural home, alongside the engine that implements it). `script.NewEngine()` already lives there.
- `Workspace` (during the transition) implements `Host` directly; `Workspace.createEntity` / `Workspace.updateEntity` invoke `runner.Process(ctx, w, autocascade.Request{...})` instead of `w.applyAutomationSideEffects(...)`. Workspace's existing `scriptExec` field stays, typed as `script.ScriptExecutor`.
- Migrate the non-Lua cascade tests in `workspace_test.go` (the 9 `TestCreateEntity_Automation*` tests listed in AC3) to `internal/autocascade/runner_test.go` with a stub Host. The five `TestLuaAutomation_*` tests stay in `workspace_test.go` as real-engine integration coverage — migrating them would force `runner_test.go` to depend on `internal/script`.

**Out of scope:**

- Building `entitymanager.Manager` as a real implementation (TKT-QTNX, separate ticket).
- Moving the rule-evaluation `automation.Engine` itself — it stays as the pure decision-maker that Runner wraps.
- Splitting `Host` into multiple sub-interfaces (entity ops / relation ops / scripting ops). Runner uses them in concert; splitting just spreads the surface across types without reducing it.
- Threading `context.Context` more deeply than today. Currently dispatch uses `context.Background()` internally; `Runner.Process` accepts ctx at entry and threads it through, but no new ctx values are added by this ticket. Better ctx-plumbing (audit `triggered_by`, principal) is a separate concern.
  - **Why `ctx` is in the signature anyway despite not being meaningfully used today:** the audit-log ticket (TKT-6YYM) needs to propagate `audit.WithTriggeredBy(ctx, ...)` through cascade execution — that's *the* place where the automation name gets stamped onto the ctx for downstream audit hooks. Accepting `ctx` now means TKT-6YYM is a pure additive change to Runner's body, not a signature change. Drop-then-re-add would force every caller to update twice.
- Removing the `wsEntityManager` adapter or moving any other Workspace internals.
- Renaming the Workspace-internal method `createEntityCore`. The Host interface uses `CreateEntityNoCascade` as the contract-side name; Workspace's implementation can forward to its existing `createEntityCore` method. The Workspace-internal rename is a follow-up if anyone cares.

**Decisions (confirmed during planning):**

1. **Package home is `internal/autocascade` (top-level).** Not inside `internal/automation`. Reasoning above.
2. **Single `Host` interface, 9 methods.** Not split into sub-interfaces. Runner uses all methods in coordinated flows.
3. **Host passed per-call to `Runner.Process`.** This is the load-bearing decision that dissolves the future cycle (EntityManager will hold Runner; EntityManager will satisfy Host; per-call dissolves the construction cycle).
4. **`lua.WriteDeps` is a field on the `Request` struct, not a Host method.** The cycle analysis below explains why.
5. **`ScriptExecutor` lives in `internal/script`** alongside the engine implementation. `script.ScriptExecutor` is the interface; `script.Engine` satisfies it. `autocascade` imports `script`. `automation` does not import `script`. *Deliberate transgression of the consumer-side-interface rule:* normally `script.ScriptExecutor` would live in `autocascade` (the consumer). Putting it in `script` is the standard Go workaround when arch-lint constraints prevent the consumer's package from importing the producer's. Calling this out so future readers don't try to "fix" it by moving the interface.
6. **`Engine` and `Scripts` are constructor fields on Runner via a `Deps` struct.** Matches the pattern TKT-QTNX uses for `entitymanager.New(Deps{...})`. Avoids positional-arg sprawl as future Runner deps are added.
7. **BFS queue stays as-is.** The dispatch is already iterative (queue-based, not recursive); Runner inherits this directly.

**Acceptance Criteria:**

1. **AC1: `autocascade.Runner` exists with documented `Host` interface.** Each method in `Host` has a comment naming where Runner invokes it. The interface is sized to *exactly* what Runner calls — no aspirational methods.
2. **AC2: Workspace's automation dispatch goes through `autocascade.Runner`.** `Workspace.createEntity` and `Workspace.updateEntity` invoke `runner.Process(ctx, w, autocascade.Request{...})`. The private methods (`applyAutomationSideEffects`, `processEntityCreations`, etc.) are deleted from Workspace. Verify via grep: no `applyAutomationSideEffects` references remain in `internal/workspace/`.
3. **AC3: Existing automation-cascade tests pass unchanged (non-Lua).** Verified test names from `internal/workspace/workspace_test.go`:
   - `TestCreateEntity_AutomationDepthLimit` (line 1106)
   - `TestCreateEntity_AutomationChainWithoutLoop` (line 1206)
   - `TestCreateEntity_AutomationWithIfExistsSkip` (line 661)
   - `TestCreateEntity_AutomationWithIfExistsError` (line 714)
   - `TestCreateEntity_AutomationWithIfExistsReplace` (line 773)
   - `TestCreateEntity_AutomationWithIfExistsUnknown` (line 828)
   - `TestCreateEntity_AutomationWithTemplate` (line 944)
   - `TestCreateEntity_AutomationWithMissingTemplate` (line 986)
   - `TestCreateEntity_AutomationWithEmptyTemplate` (line 1011)
4. **AC4: Lua-integration tests still green.** All five `TestLuaAutomation_*` tests (lines 1307, 1330, 1380, 1454, 1503, 1552) continue to pass against the real `script.Engine` via Workspace. These stay in `workspace_test.go` rather than migrating to `runner_test.go` because they exercise the actual Lua engine end-to-end and migrating them would force `runner_test.go` to depend on `internal/script`.
5. **AC5: New Runner unit tests with stub Host.** Tests in `internal/autocascade/runner_test.go` exercising Runner directly:
   - `TestRunnerDepthLimit` — stub Host produces a recursive cascade; assert the emitted warning string matches the exact wording used today (verify against `workspace.go:1082–1086` before refactor and pin the literal format string in the test). Without pinning, future refactors silently drift the wording.
   - `TestRunnerIfExistsSkip` — stub Host's `FindExistingRelationTarget` returns non-nil; assert the create action is skipped and no `CreateEntityNoCascade` call is made.
   - `TestRunnerIfExistsError` — same setup, action specifies `if_exists: error`; assert error returned with the expected message.
   - `TestRunnerActionOrder` — Result containing all four action types (PropertiesSet, RelationsToCreate, EntitiesToCreate, LuaToExecute) processed in one queue iteration; stub Host records call order; assert the order matches today's dispatch (verified against `workspace.applyAutomationSideEffects` body order before refactor).
   - `TestRunnerLuaErrorPath` — stub `script.ScriptExecutor` returns an error; assert `lua.ScriptError.Path` is patched to `"automation:<name>"` for inline-code actions (preserves the behavior at `workspace.go:1259–1262`).
   - `TestRunnerEntityCreateError` — stub Host's `CreateEntityNoCascade` returns an error; assert the error is appended to `Outcome.AutomationErrors`, the queue item is skipped, the cascade continues.
   - `TestRunnerRelationCreateError` — stub Host's `WriteRelation` returns an error; same assertion shape.
6. **AC6: `MaxDepth` constant moves to `autocascade` package.** Verify the `workspace` package no longer declares or references `maxAutomationDepth`.
7. **AC7: `just ci` green** (full pipeline: `just test`, `just lint`, `just arch-lint`, `just coverage-check`). This covers both compilation and arch-lint admission of the new `autocascade` component; no separate arch-lint AC.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing patterns referenced:**

- **CLAUDE.md "Consumer-side interfaces for callbacks and cycles"** — the pattern doc added on this branch. Worked example uses this exact ticket as motivation; the doc's example will need a light update to reflect the final package name (`autocascade` not `automation`).
- **`mcp.Services`** at `internal/mcp/server.go:45–60` — positive example of a scoped consumer-side interface that Workspace satisfies. Constructor-field form (Services held by `mcp.Server`).
- **`scheduler.WorkspaceProvider`** at `internal/scheduler/scheduler.go:39–44` — another constructor-field consumer-side interface.
- **`store.EntityObserver`** at `internal/store/store.go` — inverse-direction consumer-side interface (store invokes observers per-call).
- **`workspace.ScriptExecutor`** at `internal/workspace/workspace.go:51–61` — already a clean consumer-side interface. Moves to `internal/script.ScriptExecutor` as part of this ticket.

**Reference patterns in codebase:**

- `internal/scheduler/scheduler.go:48` — `s.ws.LuaWriteDeps()`. Scheduler currently materializes lua deps from Workspace; same pattern Runner callers will use.
- `internal/automation/engine.go:233` — `executeAction(...)` carries `automationName` through to `LuaToExecute.AutomationName`. That's the source-of-truth `automation:<name>` string Runner uses for error formatting.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical approach:**

### `Host` interface

Sized to exactly what Runner calls. 9 methods. Lives in
`internal/autocascade/host.go`.

```go
// internal/autocascade/host.go
package autocascade

// Host is what Runner needs from its caller to execute a cascade.
// Defined here, at the consumer, per CLAUDE.md "Consumer-side
// interfaces for callbacks and cycles". Host is passed per-call to
// Runner.Process so the cycle with EntityManager (which holds Runner
// and implements Host) is dissolved at the constructor level.
type Host interface {
    // Schema and storage access. Used by handleIfExists and dispatch
    // sanity checks.
    Meta() *metamodel.Metamodel
    Store() store.Store

    // Entity creation without recursive automation. Runner calls this
    // for create_entity actions; Runner then re-evaluates automations
    // on the newly created entity in its own queue.
    //
    // Workspace today has a private `createEntityCore` with similar
    // semantics; its forwarder satisfies this method.
    CreateEntityNoCascade(entityType string, opts CreateEntityOptions) (*entity.Entity, error)

    // Bare entity write (no automation). Used for property changes
    // on the trigger entity emitted by automation Result.PropertiesSet.
    WriteEntity(e *entity.Entity) error

    // Relation creation. Runner uses this for Result.RelationsToCreate
    // and for trigger relations attached to automation-created entities.
    WriteRelation(r *entity.Relation) error

    // Cascade delete with relation cleanup. Used by handleIfExists
    // when if_exists: replace deletes the existing target.
    DeleteEntity(ctx context.Context, entityType, id string, cascade bool) error

    // Existing-entity lookup for if_exists handling.
    FindExistingRelationTarget(sourceID, relationType, targetType string) *entity.Entity

    // Templating and ID generation for automation-created entities.
    Templater() templating.Templater
    GenerateID(entityType, prefix string) (string, error)
}

// CreateEntityOptions mirrors the subset of workspace.CreateOptions
// that automation-driven creations care about. Defined here so the
// Host signature doesn't leak workspace's internal options struct.
type CreateEntityOptions struct {
    ID         string
    Prefix     string
    Template   string
    Properties map[string]interface{}
    Content    string
}
```

Method naming notes:

- `CreateEntityNoCascade` (not `CreateEntityCore`) — Host's method names are what Runner calls. The "Core" suffix is a Workspace-internal naming concept; the contract-side name names the *property* (no cascade) rather than the *implementation detail* (it's the "core" of the more public method).
- `CreateEntityOptions` — defined in `autocascade`, not borrowed from workspace. Field list above is the minimum Runner actually needs; finalize during implementation by checking workspace.CreateOptions usage in the moved code.

### `Request` and `Outcome`

```go
// internal/autocascade/types.go
package autocascade

// Request is the per-invocation payload Runner.Process needs.
// Grouped into a struct to keep the Process signature readable.
type Request struct {
    // Trigger is the entity whose write initiated the cascade.
    Trigger *entity.Entity
    // OldTrigger is the trigger's prior state (nil for creates).
    OldTrigger *entity.Entity
    // Result is the automation.Result produced by Engine.Process for
    // the initiating event.
    Result *automation.Result
    // LuaDeps is the WriteDeps bundle passed through to script
    // execution. The caller materializes it; Runner just hands it to
    // the script executor. See "Avoiding the arch-lint cycle" below
    // for why this is on Request rather than on Host.
    LuaDeps lua.WriteDeps
}

// Outcome is the cumulative result of a cascade.
type Outcome struct {
    RelationsCreated   []*entity.Relation
    EntitiesCreated    []*entity.Entity
    AutomationWarnings []string
    AutomationErrors   []string
}
```

### `Runner`

```go
// internal/autocascade/runner.go
package autocascade

type Runner struct {
    engine  *automation.Engine
    scripts script.ScriptExecutor
}

// Deps is the constructor input for New. Matches the pattern used in
// entitymanager.New (TKT-QTNX) and similar focused services.
type Deps struct {
    Engine  *automation.Engine
    Scripts script.ScriptExecutor
}

func New(d Deps) (*Runner, error) {
    if d.Engine == nil {
        return nil, errors.New("autocascade: New: Engine is required")
    }
    if d.Scripts == nil {
        return nil, errors.New("autocascade: New: Scripts is required")
    }
    return &Runner{engine: d.Engine, scripts: d.Scripts}, nil
}

// Process runs the BFS automation cascade. Host receives all
// entity/relation/template callbacks. req.LuaDeps is passed through
// to script execution.
func (r *Runner) Process(ctx context.Context, host Host, req Request) (Outcome, error) {
    // BFS queue loop (lines 1057-1089 from workspace.go, parameterized over host).
    // Pre-existing per-iteration action order: Lua first, then relations, then
    // entities. Pinned by TestRunnerActionOrder (AC5).
}

// MaxDepth is the cascade depth limit. Exported so callers can verify
// against it in tests and observability.
const MaxDepth = 50
```

### Workspace's transition role

Workspace gains an `autocascade.Host` implementation. Most of Host's methods are
already exposed on Workspace; a few become small forwarders to existing private
methods:

- `CreateEntityNoCascade` → forwards to `w.createEntityCore` (existing private method).
- `WriteEntity` → forwards to `w.writeEntity` (existing private method).
- `WriteRelation` → forwards to `w.writeRelation` (existing private method).

The dispatch sites in `createEntity` / `updateEntity` change from:

```go
sideEffects := w.applyAutomationSideEffects(entity, oldEntity, autoResult)
```

to:

```go
outcome, err := w.runner.Process(ctx, w, autocascade.Request{
    Trigger:    entity,
    OldTrigger: oldEntity,
    Result:     autoResult,
    LuaDeps:    w.LuaWriteDeps(),
})
```

`w.runner` is constructed in `newWorkspace` from the existing
`automation.Engine` and the `script.ScriptExecutor` Workspace already holds
(typed via the new `script.ScriptExecutor` interface; `script.NewEngine()`
satisfies it).

### File-by-file change list

**New files:**

- `internal/autocascade/host.go` — `Host` interface, `CreateEntityOptions`.
- `internal/autocascade/types.go` — `Request`, `Outcome`, `MaxDepth`.
- `internal/autocascade/runner.go` — `Runner`, `Deps`, `New`, `Process` and the private dispatch helpers (moved from workspace).
- `internal/autocascade/runner_test.go` — stub-Host-based unit tests (AC5).
- `internal/script/executor.go` — *or wherever the script.ScriptExecutor interface ends up* — moved from `internal/workspace/workspace.go:51-78`. May replace an existing file; check during implementation.

**Modified files:**

- `internal/workspace/workspace.go` — delete the dispatch methods (`applyAutomationSideEffects` etc., lines 1049–1334), add `w.runner *autocascade.Runner` field, construct it in `newWorkspace`, add Host-implementing forwarder methods if any naming-rename is needed.
- `internal/workspace/workspace_test.go` — the five `TestLuaAutomation_*` tests stay; the nine non-Lua cascade tests delete (moved to `internal/autocascade/runner_test.go`).
- `internal/lua/deps.go` — no changes from this ticket. (`lua.WriteDeps.EntityManager` is the TKT-Y0JU concern; independent.)
- `.go-arch-lint.yml` — add `autocascade: { in: internal/autocascade }` to components. Add `deps.autocascade.mayDependOn` block (verify exact list during implementation — see "Verify mayDependOn during skeleton phase" below). Add `autocascade` to `workspace.mayDependOn`. Move `script.ScriptExecutor`-related rules if any.
- `CLAUDE.md` — update the worked example in "Consumer-side interfaces for callbacks and cycles" to reflect the final package name `autocascade` (not `automation`). One-paragraph edit.

### Verify `mayDependOn` during skeleton phase

Plan's expected list is `mayDependOn: [automation, lua, script, store,
templating]`. This is the *speculative* list based on reading the dispatch code.
The implementation order's first step is "write Host + Request + Runner
skeleton" — at that point, run `go build ./internal/autocascade/...` against the
empty skeleton and report what the compiler actually pulls in. If something
surfaces (e.g., transitive need for `project`, `state`, `filter`), add it to the
arch-lint config and document why. Don't ship arch-lint with speculative deps.

### Avoiding the arch-lint cycle via Request.LuaDeps

A naive Host design would return `lua.WriteDeps` so Runner can pass it to script
execution. But that creates a cycle in the arch-lint declarations once TKT-QTNX
(entitymanager.Manager) lands:

- `autocascade → lua` (Host returns `lua.WriteDeps`)
- `lua → entitymanager` (`lua.WriteDeps.EntityManager` field, currently in `internal/lua/deps.go`)
- `entitymanager → autocascade` (Manager holds `*autocascade.Runner`)

So `entitymanager → autocascade → lua → entitymanager`. That's a real cycle in
the arch-lint graph, even though Go-level imports work because Host is satisfied
structurally.

Putting `lua.WriteDeps` in `Request` rather than on `Host` doesn't *eliminate*
the autocascade→lua edge (the type is still referenced), but it concentrates the
dependency in one well-known place. The cycle still closes when TKT-QTNX lands —
`entitymanager → autocascade → lua → entitymanager`. That's the dependency that
TKT-Y0JU exists to break (by narrowing `lua.WriteDeps.EntityManager` to a small
`lua.EntityMutator` interface defined in `lua` itself, eliminating `lua →
entitymanager`).

**Decision:** for TKT-6OMC, accept the `autocascade → lua` edge. There's no
cycle for *this* ticket because workspace satisfies Host, and `workspace →
autocascade → lua → entitymanager` doesn't close back to workspace. The cycle
only becomes a problem when TKT-QTNX makes entitymanager the one importing
autocascade.

**Action item for ticket relations:** add `TKT-QTNX depends-on TKT-Y0JU`. The
planning artifact recording this constraint.

### Alternatives considered (rejected)

- **`internal/automation/cascade` (sub-package).** Rejected because it hides the dependency-footprint expansion under "this is part of automation" semantics. Top-level forces an explicit `mayDependOn` declaration, which the project direction favors.
- **Top-level `internal/cascade`.** Decent — but `autocascade` is more specific (named for the automation context, not generic cascading). Within rela, "cascade" alone would be ambiguous (cascading deletes are a different concept that already exists in the codebase via `DeleteEntity(cascade bool)`).
- **Top-level `internal/dispatch` or `internal/effect`.** Generic names that don't say *what* gets dispatched / effected. `autocascade` is unambiguous in code review.
- **Splitting Host into `EntityHost`, `RelationHost`, `ScriptHost`.** Runner uses all three interleaved; splitting just spreads the surface.
- **Holding Host as a Runner field rather than per-call argument.** Reintroduces the constructor cycle with EntityManager.
- **Inlining the BFS into `automation.Engine` itself.** Engine is intentionally a pure decision-maker. Mixing dispatch logic into Engine would entangle the rule-evaluator with the side-effect-haver.
- **`ScriptExecutor` stays in `internal/workspace`.** Tried; creates awkward "automation depends on workspace which depends on automation" pseudo-cycle when other consumers want it. Better home is `internal/script` alongside the engine.
- **`NewRunner(engine, scripts)` positional args instead of `Deps` struct.** Positional works for 2 args but doesn't extend gracefully. The `Deps`-struct pattern matches what TKT-QTNX uses for `entitymanager.New`. Consistency across sibling packages.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:** None new. Runner consumes the same
`automation.Result` that Workspace consumes today; validation of automation
action shape (template names, property values, etc.) happens in
`automation.Engine.executeAction` before Runner ever sees it.

**Security-sensitive operations:** None new. Lua execution path is unchanged —
same `script.ScriptExecutor` invoked with the same `WriteDeps`. Template name
validation (`isValidTemplateName` at `automation/engine.go:360`) is upstream of
Runner and not affected.

**Error handling:** No change to error surfacing. `automation.Result.Warnings` /
`Result.Errors` flow into `Outcome.AutomationWarnings` / `AutomationErrors`
identically. The Lua error-path patching at `workspace.go:1259–1262` (sets
`lua.ScriptError.Path` to `"automation:<name>"` for inline-code actions) moves
into Runner verbatim and is pinned by `TestRunnerLuaErrorPath` (AC5).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Integration test approach defined

**Test scenarios:**

- **AC1, AC2, AC6:** verified by `go build ./...` (compilation) + `grep` for residual references.
- **AC3:** existing cascade tests in `internal/workspace/workspace_test.go` pass unchanged (they exercise Workspace, which now delegates to Runner).
- **AC4:** all five `TestLuaAutomation_*` tests pass against real `script.Engine` (stay in workspace_test.go).
- **AC5:** new tests in `internal/autocascade/runner_test.go` — listed in AC5 body.
- **AC7:** `just ci` green (includes arch-lint).

**Edge cases:**

- Empty `automation.Result` → Runner.Process returns empty `Outcome`, no Host calls.
- Result with only `PropertiesSet`, no entity/relation creations → Runner calls `Host.WriteEntity` with mutated trigger entity, no further queue items.
- Result with both Lua and entity creations → Lua executes first within the queue iteration, then relations, then entity creations (matching workspace's current order — pinned by `TestRunnerActionOrder`).
- Cascade hits depth limit → warning emitted with the exact format string today's code uses (pinned by `TestRunnerDepthLimit`), queue drained gracefully, no panic.
- Lua script returns an error → recorded in `Outcome.AutomationErrors` with patched path; cascade continues with next action.
- Host method returns an error → behavior depends on which method. Verified against today's code:
  - **`WriteEntity` / `CreateEntityNoCascade` failure during `processEntityCreations`** (`workspace.go:1099–1127`): error is appended to `effects.AutomationErrors`, the queue item is skipped, cascade continues with the next item. Pinned by `TestRunnerEntityCreateError` (AC5).
  - **`WriteRelation` failure during `applyRelationCreations`** (`workspace.go:1170–1199`): same pattern — error appended, continue. Pinned by `TestRunnerRelationCreateError` (AC5).
  - **Property-set failure** (during `runCreatedEntityAutomation`'s `WriteEntity` for `Result.PropertiesSet`): same — appended, continue.
  - **`ScriptExecutor.ExecuteCode/File` failure during `executeLuaActions`** (`workspace.go:1239–1245`): appended to errors with patched `lua.ScriptError.Path`, continue. Pinned by `TestRunnerLuaErrorPath` (AC5).

All four paths share "continue, don't abort the cascade" semantics today. The
plan preserves this. The behavior-pinning protocol below applies to all of them.

**Behavior pinning (separate from "is the refactor correct"):**

The following behaviors are not currently covered by focused tests; the new
Runner tests fill that gap. Before deleting the old code, run each new test
against the *unrefactored* code path to verify the test pins existing behavior
(i.e., the test passes against today's workspace.go), then refactor:

- Lua error-path patching at `workspace.go:1259–1262` → `TestRunnerLuaErrorPath`.
- Depth-limit warning wording at `workspace.go:1082–1086` → `TestRunnerDepthLimit`.
- Per-iteration action order in `applyAutomationSideEffects` → `TestRunnerActionOrder`.
- Error-continuation semantics across all four action paths → `TestRunnerEntityCreateError`, `TestRunnerRelationCreateError`.

This protocol catches drift even when the refactor is otherwise mechanical.

## Risk Assessment

- [x] Technical risks assessed
- [x] Effort estimated

**Risks:**

| Risk | Mitigation |
|---|---|
| Behavior drift during the move (subtle reorderings, edge-case omissions) | Pin behavior with `TestRunnerActionOrder`, `TestRunnerLuaErrorPath`, `TestRunnerDepthLimit`, `TestRunnerEntityCreateError`, `TestRunnerRelationCreateError` *before* deleting the old code. Run each new test against the unrefactored code first to verify it pins existing behavior. |
| Test coverage drops if non-Lua cascade tests move but workspace_test.go's coverage floor depends on them | Before moving, run `just coverage-check` and record current `workspace` floor. After move, check the same. If workspace's floor drops, leave a smoke test in workspace_test.go that delegates the assertion. |
| TKT-QTNX cycle problem deferred to TKT-Y0JU | Add `TKT-QTNX depends-on TKT-Y0JU` to the ticket graph. Recorded as an action item above. |
| `CreateEntityOptions` field list incomplete — implementation discovers `workspace.CreateOptions` carries more state than expected | Walk through `workspace.createEntityCore` (line 935-999) during implementation; populate `CreateEntityOptions` to match. If new fields are needed, add them — they're a contract between two packages we're both writing. |
| arch-lint expansions reveal a transitive dependency not surfaced in planning | Run `go build ./internal/autocascade/...` against the empty skeleton (first step of implementation) and let the compiler surface the real list. The plan's speculative list is `[automation, lua, script, store, templating]`; adjust if `project`, `state`, etc. surface. |
| `script.ScriptExecutor` interface move from workspace breaks other consumers of `workspace.ScriptExecutor` | Grep for references: `workspace.ScriptExecutor`, `workspace.NopScriptExecutor`. Update each import. Keep `workspace.NopScriptExecutor` as a re-export or move it to `script.NopScriptExecutor`. |

**Effort: m.** Rough breakdown: ½ day write Host + Request + Runner skeleton +
run `go build ./internal/autocascade` against the empty skeleton to verify the
real `mayDependOn` list; 1 day move dispatch code in, parameterize over Host; ½
day Workspace forwarders + dispatch site update; ½ day move `ScriptExecutor` to
`internal/script` + arch-lint; 1 day migrate tests + write new Runner tests; ½
day docs + CLAUDE.md edit + cleanup. ~3.5 days end-to-end.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [ ] User guide / reference docs — N/A (internal refactor).
- [ ] CLI help text — N/A.
- [x] CLAUDE.md — update the worked example under "Consumer-side interfaces for callbacks and cycles" to use the real `autocascade.Host` rather than the hypothetical `automation.Host`. Light edit.
- [ ] README.md — N/A.
- [ ] API docs — N/A.

## Design Review

- [x] Plan reviewed (`/crit` round 1 — 1 finding, addressed; cranky + go-architect agent reviews stalled, inline reviews completed instead — 5 significant + 5 minor findings, all addressed)
- [x] All critical/significant findings addressed in plan

**Design Review Findings (round 1):**

- **crit c_a41c3c** ("This is a hint that we should probably make a new module for the runner") — addressed by moving Runner to `internal/autocascade` (top-level), not `internal/automation`. Plan rewritten throughout.
- **Cranky-inline-critical** (10-method Host returning `lua.WriteDeps`) — addressed by moving `lua.WriteDeps` to `Request.LuaDeps` field.
- **Cranky-inline-significant** (ScriptExecutor home indecision) — locked in: `internal/script`.
- **Architect-inline-significant** (`Runner.Process` six positional args) — addressed by `Request` struct.
- **Architect-inline-significant** (`NewRunner` positional args) — addressed by `New(Deps{...})` pattern, matching TKT-QTNX.
- **Cranky-inline-significant** (`CreateEntityCore` Workspace-internal name leakage) — addressed by `CreateEntityNoCascade` naming + `CreateEntityOptions` defined in autocascade.
- **Cranky-inline-significant** (action-order test missing) — addressed by `TestRunnerActionOrder` (AC5).
- **Cranky-inline-significant** (Lua-error-path patching not covered) — addressed by `TestRunnerLuaErrorPath` (AC5).
- **Cranky-inline-minor** (depth-limit warning wording not pinned) — addressed by requiring `TestRunnerDepthLimit` to assert the exact format string.
- **Cranky-inline-minor** (AC8 redundant with AC7) — addressed by folding into AC7.
- **Cranky-inline-minor** (error-continuation paths claim is hand-waved) — addressed by walking all four paths in the Test Plan and adding `TestRunnerEntityCreateError` + `TestRunnerRelationCreateError` (AC5).
- **Architect-inline-minor** (`ctx` parameter justification missing) — addressed by an explicit sub-bullet under "Out of scope" explaining why ctx is in the signature.
- **Architect-inline-minor** (arch-lint dep list speculative) — addressed by adding a "Verify mayDependOn during skeleton phase" subsection and updating the effort breakdown.
- **Architect-inline-minor** (consumer-side rule transgression for `script.ScriptExecutor` not named) — addressed by explicit call-out under Decisions #5.

Round 2 review pending after these edits.
