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
Lua script execution — lived inside `internal/workspace/workspace.go` as a
cluster of private methods (lines 1049–1310 pre-refactor). Workspace is being
decomposed (CLAUDE.md flags it as a transitional shim). Lifting this dispatch
into its own service is the foundational step for FEAT-workspace.

The refactor applies the consumer-side-interface pattern. The new `autocascade`
package declares a `Host` interface naming exactly the methods Runner calls back
into; Workspace implements `Host` during the transition; future
`entitymanager.Manager` will implement it later.

### Package home: `internal/autocascade` (shipped)

Top-level package (not inside `internal/automation`). `autocascade.Runner` is
the runner for automation cascades. Justification:

- Small services with explicit dependencies; sub-package would hide dep-footprint expansion under "this is part of automation."
- `internal/automation` stays as a pure rule evaluator (`mayDependOn: [filter, metamodel]` unchanged).
- `internal/autocascade` is the executor (mayDependOn: `[automation, lua, metamodel, store]` — see post-review-2 update below).

**In scope (shipped):**

- New `internal/autocascade/` package: `Host` (7 methods), `Request`, `Outcome`, `MaxDepth = 50`, `Runner` with `New(Deps{...})` constructor.
- `autocascade.Executor` interface + `NopExecutor` (post-review-2 move from `internal/script`).
- Workspace dispatch code (lines 1049–1310 pre-refactor) moved into Runner. Workspace satisfies `autocascade.Host` via `internal/workspace/autocascade_host.go` forwarders.
- Workspace's `createEntity` / `updateEntity` invoke `runner.Process(...)` instead of the now-deleted dispatch methods.
- 10 Runner unit tests with stub Host (`internal/autocascade/runner_test.go`).
- 14 existing workspace cascade tests pass unchanged (9 non-Lua + 5 Lua-integration).
- workspace.ScriptExecutor / NopScriptExecutor remain as type aliases for back-compat with CLI callers.

**Out of scope (unchanged):**

- Building `entitymanager.Manager` as a real implementation (TKT-QTNX).
- Moving the rule-evaluation `automation.Engine` itself.
- Removing the `wsEntityManager` adapter.

**Decisions (shipped):**

1. **Package home is `internal/autocascade` (top-level).**
2. **`Host` shipped 7 methods, not 9.** During implementation it became clear that `Templater()` and `GenerateID()` are not called by Runner — they're called *inside* `Host.CreateEntityNoCascade` (which forwards to `workspace.createEntityCore`). The original plan over-specified Host based on a survey that mis-attributed dependencies. The shipped interface is sized to *exactly* what Runner calls.
3. **Host passed per-call to `Runner.Process`** — dissolves the future cycle with EntityManager.
4. **`lua.WriteDeps` is a field on the `Request` struct, not a Host method.**
5. **`script.Executor` was originally placed in `internal/script`, then moved to `internal/autocascade` after the round-2 review.** The "consumer-side rule transgression" the plan documented turned out to be unnecessary: `script` doesn't import `autocascade`, so `autocascade` can own the interface that `*script.Engine` happens to satisfy structurally. The plan's Decisions #5 over-defended a non-existent cycle.
6. **`Engine` and `Scripts` are constructor fields on Runner via a `Deps` struct.**
7. **BFS queue stays as-is.**

**Acceptance Criteria (all met):**

1. ✅ **AC1:** `autocascade.Runner` exists with documented `Host` interface (7 methods, each documented at its call site).
2. ✅ **AC2:** Workspace's automation dispatch goes through `autocascade.Runner`. Original dispatch methods are deleted; grep confirms no `applyAutomationSideEffects` references remain in workspace.
3. ✅ **AC3:** 9 non-Lua cascade tests in `workspace_test.go` pass unchanged.
4. ✅ **AC4:** 5 `TestLuaAutomation_*` tests pass against real `script.Engine`.
5. ✅ **AC5:** 10 new tests in `internal/autocascade/runner_test.go`:
   - `TestNew_RejectsNilEngine`, `TestNew_RejectsNilScripts` — constructor validation
   - `TestRunnerEmptyResult` — fast path
   - `TestRunnerDepthLimit` — pins warning format string
   - `TestRunnerIfExistsSkip` / `TestRunnerIfExistsError`
   - `TestRunnerEntityCreateError` / `TestRunnerRelationCreateError`
   - `TestRunnerActionOrder` — pins Lua → relations → entities ordering
   - `TestRunnerLuaErrorPath` — pins `lua.ScriptError.Path` patching
6. ✅ **AC6:** `MaxDepth = 50` lives in `autocascade`; `workspace` no longer declares `maxAutomationDepth`.
7. ✅ **AC7:** `just ci` green: lint, arch-lint, race-enabled tests, coverage.

## Research, Approach, Security, Test Plan, Risk Assessment, Documentation Planning

(See git history for the original detailed planning sections. Substantive
content is preserved in the codebase via doc comments. Implementation revealed
two corrections to the plan, both folded above:)

- **Host shrank from 9 → 7 methods** (Templater/GenerateID not actually called by Runner).
- **`script.Executor` moved from `internal/script` → `internal/autocascade`** during round-2 review (the "documented transgression" was over-defensive; no cycle existed).

## Design Review

- [x] Plan reviewed (`/crit` round 1 — 1 finding, addressed; cranky + go-architect inline-reviews round 1 — 5 significant + 5 minor findings, all addressed; cranky + go-architect agent reviews round 2 on the *implementation* — 1 critical + 5 significant + 6 minor findings)
- [x] All critical/significant findings addressed in code

**Design Review Findings (round 2 — on shipped implementation):**

Critical:
- **Cranky #1 (silent automation disable):** `newWorkspace` was swallowing `autocascade.New` failure and disabling automation with a warning. Fixed: `newWorkspace` returns `(*Workspace, error)`, propagates failure; `NewForTest` panics (matches existing convention there).

Significant (all addressed):
- **Architect #5 (nil Trigger guard):** Added `req.Trigger == nil` check in `Runner.Process`, returns explicit error.
- **Architect #11 (ctx not threaded):** `applyRelationCreations` now takes `ctx` and uses it instead of hardcoded `context.Background()`.
- **Architect #7 (Executor placement):** Moved from `internal/script` to `internal/autocascade`. The architect was right: the cycle the plan worried about doesn't exist; the consumer should own its interface.
- **Cranky #5 (stub upsert hack):** Stub Host's `WriteEntity` now properly checks for `store.ErrConflict` rather than swallowing any error.
- **Cranky #6 (FindExistingRelationTarget promotion):** Added a comment on the method in workspace.go explaining *why* it's exported (not just *that* it was promoted).

Minor (folded):
- **Cranky #2 (plan/code drift on Host method count):** Plan now records the shrink from 9 to 7 methods explicitly.
- **Cranky #4 (Outcome.Errors doc apologetic):** Tightened the comment — same content, less hand-wringing for an exported type.
- **Cranky #9 (stale ScriptExecutor doc):** Replaced the "workspace defines the interface" line with a concise re-export note.
- **Cranky #10 (host_forwarder typo):** Caught and fixed during the C1 edit; no `host_forwarder.go` references remain.

Deferred (out of scope for TKT-6OMC, flagged for follow-up):
- **Cranky #3 (OldTrigger propagation test):** The behavior is documented as preserved; integration tests cover it implicitly. Adding a focused test for "weird but preserved" behavior is over-pinning.
- **Cranky #7, #8 (DeleteEntity entityType param, NopExecutor goroutine note):** Bikeshed-level.
- **Cranky #11 (BFS slice growth):** Cap-50 cascade, not worth touching now.
- **Architect #6 (Outcome parallel slices):** Confirmed flag-for-follow-up.
