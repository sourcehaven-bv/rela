<!-- @managed: claude-workflow v1 -->
---
id: IMPL-ERHWL0
type: implementation-checklist
title: 'Implementation: Memoize dashboard breakdown and table-row derivation'
status: done
---

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no
      cross-component flow — one component's internal derivation, no API, store
      or router involvement changed. The component-level tests mount the real
      view and drive it through the real template, which is the full flow here.)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Two `computed` maps (`breakdowns`, `tableRows`) keyed by `cardKey(card)`;
`getBreakdown` / `getTableRows` reduced to lookups with a `|| []` fallback for a
card whose search has not yet resolved. All three planning edge cases covered:
missing `group_by`, key absent from `cardData`, and data landing after first
render.

No error handling to add or remove — these derivations are pure reductions over
already-fetched data. `loadData`'s existing try/catch is untouched.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Reused the file's existing `card()` / `seedDashboard()` factories from #1316
rather than adding a parallel set. Added a local `entity()` helper for row
fixtures. The read-count assertion is `expect(reads).toBe(rows.length)` — derived
from the fixture, not the literal `3`, so it stays correct if the fixture grows.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

AC1 — breakdown derived once. A getter on `status` counts property reads; 3
entities through two template call sites yields **3 reads**. Mutation-tested:
reverting `getBreakdown` to its non-memoized form fails with
`AssertionError: expected 6 to be 3`, confirming the guard bites and that the
doubling was real. Re-confirmed after the rebase onto develop.

AC2 — table rows derived once. A card sorted `title asc` over input
`['c','a','b']` renders `['a','b','c']`.

AC3 — no output change. The 7 pre-existing #1316 cases pass unmodified.

Edge case (data arriving post-render): mount without awaiting the search, assert
the value is absent, resolve the promise, assert it appears. This is the memo
invalidating against a `.set()`-mutated Map inside a `ref`.

Full suite after rebase: **1665 tests / 105 files pass**. `vue-tsc --noEmit`
clean. `npm run build` clean. `npm run lint` 0 errors, no warnings in the
changed files.

One test I wrote was wrong and was replaced: it asserted a refetch after
swapping the card list, but `loadData` runs only from `onMounted`, so nothing
refetches. That is pre-existing behaviour unrelated to this ticket (noted in the
review checklist); the replacement exercises the real invalidation path.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
      patterns extracted to a helper / constant / type where it
      sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the precomputed-rows pattern `frontend/CLAUDE.md` prescribes for dense
surfaces (RR-UD2A). Extracted a `Breakdown` type alias, which was previously an
inline structural type repeated in the signature — that one sharpens the
contract and is used by both the computed and the getter. Did not extract the
two map-building loops into a shared generic helper: they share shape but not
logic (a group-by vs. a sort-and-slice), and merging them would obscure both.

`cardKey` reused from #1316 rather than duplicated.
