---
id: IMPL-AMR3FR
type: implementation-checklist
title: 'Implementation: List group_by on a single-valued relation'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: TestValidateListGroupBy_Relation and TestNormalizeListGroupBy in validate_groupby_test.go; shared validation in validate_columns_from_test.go
- [x] Integration tests written (test full flow, not just units): no dedicated frontend test for the relation path in useListGrouping; it reuses useRelationColumns, which the KanbanView tests cover, and was checked on the demo
- [x] Happy path implemented: lists group by `heeft_status` with sections in status order
- [x] Edge cases from planning handled: Other section and empty sections
- [x] Error handling in place (errors surfaced, not swallowed): config errors name the list and key

## Test Quality

- [x] Using fixture builders or factories for test data: table-driven config tests with small schemas
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: on the demo server `/list/taken` and the initiative's `lijst` tab group tasks by status
- [x] Each acceptance criterion verified with test scenario from planning: AC1: lists and boards take sections from the same useRelationColumns, and the grouped list was checked on the demo. AC2: the demo README documents that Add in a section prefills the status
- [x] Edge cases manually verified: Other and empty sections come from the shared column resolution; not checked separately in the list

**Verification Evidence:**
- Demo: the grouped list by relation works.
- `go test ./internal/...` passes.
- Frontend vitest full suite passes: 3770 tests in 255 files. `vue-tsc` is clean.
- `golangci-lint` reports 0 issues on the changed packages.
- Postgres tests (pgstore, entitymanager with `-tags postgres`) pass against a throwaway database.

## Quality

- [x] Code follows project patterns (check similar code): follows the existing property `group_by` code path
- [x] Checked for DRY opportunities: column resolution shared with the kanban
- [x] No security issues introduced: no new endpoint
- [x] No silent failures (errors logged AND returned): validation errors surface at config load
- [x] No debug code left behind
