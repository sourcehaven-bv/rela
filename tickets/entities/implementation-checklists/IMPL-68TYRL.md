---
id: IMPL-68TYRL
type: implementation-checklist
title: 'Implementation: Query-budget test for the nested view section'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — three tests in
      `internal/dataentry/querybudget_test.go`, plus the `storetest.Breadth`
      decorator they need.
- [x] Integration tests written (test full flow, not just units) — all three
      drive the real HTTP handler via `viewsAs`, through the real ACL gate and
      the real view pipeline. No test calls `buildNestedTree` directly.
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place~~ (N/A: test-only change, no new error paths)

**What was built:**

| File | Change |
| --- | --- |
| `internal/store/storetest/breadth.go` | NEW. `Breadth` store decorator recording `len(EntityQuery.IDs)` and `len(RelationQuery.EntityIDs)` per read. |
| `internal/dataentry/querybudget_test.go` | `program` type + `in-program`/`in-epic` relations + a `display: nested` view in the fixture; three new tests; two new constants. |

No production code changed. `internal/store/storetest` is test infrastructure,
not shipped code.

**Why a new decorator rather than extending `Counting`:** the two answer
different questions. `Counting` counts round-trips, which is what an N+1
inflates; `Breadth` measures how wide each round-trip is, which an over-fetch
inflates while leaving the count untouched. Keeping them separate also means
the four existing pins (6, 11, 3, 12) cannot move even in principle.

## Test Quality

- [x] Using fixture builders or factories for test data — extends the existing
      `budgetMeta`/`budgetConfig`/`newBudgetApp` factories rather than adding a
      parallel set.
- [x] No hardcoded values in assertions when object is in scope — the breadth
      test's fixture size is DERIVED from `nestedRelationIDLimit` and
      `nestedChildPreview`, not hardcoded.
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

**On the derived sizing.** A hardcoded parent count was tried and rejected
after it silently broke the guard: at 110 parents the visible universe (3,300
ids) no longer exceeded the 4,000-id limit, so the test passed with the defect
present. It now computes its own size and asserts its own adequacy — if the
visible universe cannot exceed the limit, the test fails with "fixture cannot
trip the guard" rather than passing vacuously.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

**AC1 — size independence.** Instrumented the response to confirm the fixture
scales on both levels, not just one:

```
n=10  parents= 2  children=10  reads=15
n=50  parents=10  children=50  reads=15
```

Parents 2→10 and children 10→50 while reads stay flat at 15. The earlier
version of this fixture held parents at 1 and was corrected (design review
RR-S4); a per-parent N+1 would have been invisible.

**AC2 — pinned constant.** `nestedSectionBudget = 15`. Attributed by
measurement, not derivation: the same view with the relation column removed
measures 12, so the column costs exactly 3 — two `ListRelations` (one per row
type, epic and ticket) plus one `ListEntityHeaders`. Breakdown at both sizes:

```
GetEntityState=1 ListEntities=4 ListEntityHeaders=1 ListRelations=9
```

**AC3 — RR-HKHPYG guard.** Mutation-verified in both directions by
reintroducing the real defect in `buildNestedTree` (resolving over
`result.Collections[sec.Children]` instead of the planned rows):

| Code | `ListRelations` ids | Result |
| --- | --- | --- |
| defect present | 8,911 | **FAIL** — "resolved over 8911 ids to render at most 2000 rows" |
| defect absent | ~2,400 | PASS |

Production code restored and `git diff` on `sections_nested.go` confirmed empty
after each mutation.

**Fixture validity (not an AC; design review RR-S3).** `newAppFromParts`
publishes the schema directly and never calls `ValidateConfig`, so every budget
fixture could pin a number for a config no operator could load.
`TestQueryBudget_FixtureConfigIsValid` closes that for all five fixtures.
Mutation-verified by setting `children: epics` (nesting a collection under
itself):

```
budget fixture config is not loadable: view "nested": section[0] nests
collection "epics" under itself (children must differ from source)
```

**Two wrong assertions found and corrected during verification**, both of which
would have shipped a test that passes regardless of the defect:

1. Asserting on `ListEntityHeaders` — it recorded **1 id**. `visibleTitles`
   only takes the header path when the view reader implements
   `visibility.HeaderFilterer`, and falls back to `ListEntities` otherwise;
   this fixture takes the fallback.
2. Including `ListEntities` in the sum — it is **byte-identical** (17,112 ids)
   with and without the defect, because the view pipeline loads each collection
   in full before any section builder runs. Including it made the limit
   untrippable.

Only `ListRelations` tracks the fix, which is what the test now asserts on.

## Quality

- [x] Code follows project patterns — `Breadth` mirrors `Counting`'s structure
      (embedded `store.Store`, mutex-guarded maps, `Reset`/`String`, forwarded
      header capability and `Tx` view) so the two read as siblings.
- [x] Checked for DRY opportunities — `Breadth` deliberately does NOT reuse
      `Counting`'s internals. They share shape but not meaning, and coupling
      them would let a change to one perturb four pinned budgets in the other.
      `itoa` is reused from the package.
- [x] No security issues introduced — test-only. The fixture grants
      `read: ["*"]`, so gating is not under measurement; the test comment says
      so explicitly, since a cost pin must not be read as evidence about the ACL.
- [x] No silent failures — every seeding error goes through `must`.
- [x] No debug code left behind — the two temporary probe files
      (`zz_probe_test.go`) and the `budgetConfigHook` seam used to attribute the
      relation-column cost were removed; `grep` for both returns nothing.

**Test cost.** The breadth fixture builds ~8,300 entities, which is heavy for a
unit test. First draft added ~18s to a 33s package; the derived sizing brought
the package to ~37s. The fixture cannot be made much smaller without ceasing to
trip the guard, which is the constraint that sets the floor.
