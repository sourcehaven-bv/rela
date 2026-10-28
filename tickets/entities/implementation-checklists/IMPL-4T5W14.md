---
id: IMPL-4T5W14
type: implementation-checklist
title: 'Implementation: List export reads relation columns in one batched query'
started: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Ran the seqtrace postgres demo (`SEQTRACE_SUITE=routes`, 550 tasks) on this
  branch. `GET /api/v1/tasks/_export?transform=txt` as the reader issued 11
  queries, against 1,107 on develop (1,100 of them per-row `ListRelations`).
- The 57,486-byte export body is byte-identical (`cmp`) to the develop run.
- `TestQueryBudget_ListExportIsSizeIndependent`: 6 store reads at 10 and 50
  rows under both the fixture and the realistic ACL; before the change it was
  15 and 55 reads.
- `TestExport_List_RelationColumnsGolden` passed on the per-row code before
  the change and on the batched code after it.
- The `export-list.*` golden files changed only in their `reads:` and
  `breadth:` lines; every body is unchanged.
- Edge cases: self-loop, edge between two exported rows, shared neighbor,
  hidden-only cell (golden test); faced row (`TestExport_List_FacedRowCarriesOnlyItsOwnEdges`);
  store error (`TestExport_List_EdgeReadFaultFailsTheExport`); duplicate ids
  and directions (`TestPageRelationsDir`).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
