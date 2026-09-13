---
id: TKT-M0WMEE
type: ticket
title: Query-budget test for the nested view section
kind: test
priority: medium
effort: s
status: ready
---

## Description

Pin the store-read cost of `display: nested` with a `storetest.Counting` budget
test, asserting the count is identical at 10 and 50 parent rows.

This is AC7 of TKT-MJKZQ3, which shipped without it. Root `CLAUDE.md` is
explicit: *"New read paths pin their cost with a `storetest.Counting` budget
test asserting the count is the same at 10 and 50 rows."* The nested section is
a new read path and has no such test.

## Why it matters more than a missing test usually would

Code review of TKT-MJKZQ3 found a real cost defect that this test would have
caught (RR-HKHPYG): `buildNestedTree` resolved relation columns over **every
visible child** rather than the ones the budget would emit, so a 1,400-epic
project with 200 tasks each passed ~280,000 ids to `RelationQuery.EntityIDs` to
render at most 2,000 rows. That is fixed — `planNestedRows` now selects first,
so `resolveRelationColumns` sees at most `nestedNodeBudget` entities — but the
fix is argued by inspection, not enforced.

The specific thing inspection did not settle cheaply: `buildNestedEntityData`
runs **per node** inside the emit loop, and whether it issues per-row I/O is
exactly what a Counting test answers.

## What to write

Extend `internal/dataentry/querybudget_test.go`, which already has the harness:

- `newBudgetApp` (`:83`) wraps a memstore in `storetest.NewCounting`.
- `readsFor` (`:136-147`) runs an op at `[]int{10, 50}` rows.
- `assertBudget` (`:149-156`) fails when the two differ, and separately pins the
exact constant.

Follow `TestQueryBudget_ListPageIsSizeIndependent` (`:162`, pinned at 6) and
`TestQueryBudget_ViewTableSectionIsSizeIndependent` (`:178`, pinned at 11). The
view-table case is the closer model, since a nested section resolves relation
columns the same way.

The fixture needs a two-step traverse (parent → child) with at least one
relation column, so the test exercises `resolveRelationColumns` rather than only
property cells.

## Acceptance criteria

1. A budget test asserts the nested section's store reads are equal at 10 and 50 parent rows.
2. The exact count is pinned as a named constant beside the existing ones, with the per-read breakdown in a comment (as `listPageBudget` does).
3. The test fails if relation-column resolution is moved back ahead of the budget (the RR-HKHPYG regression).
