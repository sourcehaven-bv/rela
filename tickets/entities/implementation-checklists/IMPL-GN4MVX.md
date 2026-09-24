---
id: IMPL-GN4MVX
type: implementation-checklist
title: 'Implementation: sqlitestore: push GraphQuery down into SQL like pgstore'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (EXPLAIN QUERY PLAN tests, Reconcile lifecycle tests)
- [x] Integration tests written (storetest.RunAll and RunGraphDifferential on sqlite; appbuild boot test; dataentry scope comparison and budgets on sqlite)
- [x] Happy path implemented
- [x] Edge cases from planning handled (unsafe JSON-path names fall back to graphquerynaive; FaceIn before the rank; Props/Narrowing on the prime; empty desired set drops only owned indexes)
- [x] Error handling in place (errors surfaced, not swallowed; boot reconcile logs and continues, `rela db reconcile` returns the error)

## Test Quality

- [x] Using fixture builders or factories for test data (seeded generator; newBudgetAppOn)
- [x] No hardcoded values in assertions when object is in scope (the differential compares with graphquerynaive)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- AC1: `go test -tags sqlite ./internal/store/sqlitestore/` passes the conformance suite on the SQL path.
- AC2: TestGraphDifferential passes on sqlite (333 of 400 queries take the SQL path) and on pgstore.
- AC3: EXPLAIN QUERY PLAN tests show `rela_derived_query__` and `rela_derived_list__` in use with no `USE TEMP B-TREE`. Endpoint matches and MatchingIDs are index-only; MatchingIDs is driven by the page's id list.
- AC4: TestQueryBudget_ListPageIsSizeIndependent_SQLite and TestQueryBudget_TraversalScopeIsPushedDown_SQLite hold the pinned counts at 10 and 50 rows.
- Manual: `rela-sqlite db reconcile --dry-run` on a temp project reported "would create", exited 1; a real run created the index; a second dry run reported up to date; removing data-entry.yaml reported "would drop".

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (all values bound; JSON paths are literals only for names that pass safeLiteral, else naive fallback)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
