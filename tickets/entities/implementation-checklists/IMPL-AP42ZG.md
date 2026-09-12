---
id: IMPL-AP42ZG
type: implementation-checklist
title: 'Implementation: executeView traversal is ungated: hidden intermediaries leak reachable descendants in _views API, entity-detail sections, and view commands'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Unit-level: `executeView` traversal, `where:`-probe, entry gate, NopACL
regression. Integration-level: the real `_views` wire handler
(`handleV1Views`) and the side panel (`handleV1SidePanel`), both asserted on
the serialized JSON body so a leak is caught at the wire, not just in the
collection.

Error handling is **fail-closed** at every gate: a header-scan fault drops the
frontier (and logs), an id whose type cannot be resolved is dropped, and a gate
probe error drops that type. A read-ACL failure must never widen visibility.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Fixture built via `testutil.EntityFor` / `testutil.NewRelation` builders and a
shared `traversalLeakApp()` factory. The wire assertions match on the fixture's
own id and title through one `leaksVisibleDescendant` helper rather than
repeating literals per test.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Each gate was disabled in isolation and the suite re-run, to prove the fix is
load-bearing rather than incidentally green.

1. **Frontier gate removed** (`frontier = next`), load gate left in place —
   three tests FAIL:

   ```text
   --- FAIL: TestACLViewTraversal_HiddenIntermediaryBlocksReachability
       LEAK: TKT-VISIBLE reachable only via hidden SEC-HIDDEN surfaced: [TKT-VISIBLE]
   --- FAIL: TestACLViewTraversal_ViewsAPIDoesNotLeakReachableDescendant
       LEAK: _views response exposed descendant reachable only via hidden node
   --- FAIL: TestACLViewTraversal_SidePanelDoesNotLeakReachableDescendant
       LEAK: _sidepanel exposed descendant reachable only via hidden node
   ```

   The leaked `_views` body contained the full `TKT-VISIBLE` section entity.
   This is the proof that **the load gate alone does not close the hole**:
   `TKT-VISIBLE` is readable in its own right, so neither the load gate nor the
   out-gate drops it.

2. **Both gates in place** — all pass:

   ```text
   --- PASS: TestACLViewTraversal_HiddenIntermediaryBlocksReachability
   --- PASS: TestACLViewTraversal_NopACLReturnsFullChain
   --- PASS: TestACLViewTraversal_WhereCannotProbeHiddenProperty
   --- PASS: TestACLViewTraversal_EntryGateBlocksHiddenEntry
   --- PASS: TestACLViewTraversal_ViewsAPIDoesNotLeakReachableDescendant
   --- PASS: TestACLViewTraversal_SidePanelDoesNotLeakReachableDescendant
   ```

3. **Load gate removed**, frontier gate in place — the ACL suite still passes,
   so the load gate is **defense in depth, not independently load-bearing** for
   these surfaces. Its distinct effect was measured directly rather than
   assumed: calling `loadViewEntities(ctx, ["SEC-HIDDEN","TKT-VISIBLE"])` as
   `alice` returns `[SEC-HIDDEN TKT-VISIBLE]` without the gate and
   `[TKT-VISIBLE]` with it. It is kept because it covers the non-recursive
   path, which never enters the BFS.

Acceptance criteria, each verified:

- `_views` traversal through a hidden intermediary drops descendants reachable
  only via it — evidence 1/2 above.
- Entity-detail sections behave the same — the side-panel test.
- A `where:` clause cannot infer a hidden entity's property values —
  `TestACLViewTraversal_WhereCannotProbeHiddenProperty`.
- NopACL output unchanged — `TestACLViewTraversal_NopACLReturnsFullChain`.
- The "chokepoint" comment is now true — corrected in `views_handler.go` to
  state that the entry and the traversal are gated separately.
- A prevention measure is added — see below.

Full `internal/dataentry` package passes (`ok ... 102s`). Two failures seen
first were stale gitignored frontend build artifacts
(`app_editor_dist/`, `static/v2/`) in the working tree, not a regression —
confirmed by a clean `origin/develop` worktree where the same tests pass, and
resolved by clearing the artifacts.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the existing batched-gate pattern used by `gateOriginSources`
(`history_origin.go`) and the neighbor-title gate in `api_v1.go`: group ids by
type, one `PermitsReadMany` per type, fail closed on error. Uses
`store.ListEntityHeaders` for id→type resolution so the walk keeps its
"no entity loads during traversal" property (TKT-1U8XYN).

DRY: the frontier filter is extracted as `readableViewIDs` rather than inlined,
because it is the named contract the lint tripwire and the godoc both point at.
The load gate is deliberately NOT folded into it — it already holds loaded
entities and so needs no header scan; sharing would mean re-resolving types it
already has.

Prevention (satisfies the bug's prevention measure): `TestViewTraversalIsSourceGated`
in `lint_test.go` is a structural tripwire over BOTH gate sites. It has no
"guard is moot" skip branch on purpose — the original version of this guard
keyed a skip on `views.go` containing `GetEntity`, and the very next refactor
moved that read into `viewworld.go`, which would have turned the guard into an
unconditional pass while the hole reopened.
