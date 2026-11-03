---
id: IMPL-6YCCMH
type: implementation-checklist
title: 'Implementation: Kanban columns from a single-valued relation (columns_from)'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: validate_columns_from_test.go (config), KanbanView.columnsFrom.test.ts (Add per column, New without prefill, drop re-point, colours), DynamicForm.prefill.test.ts (single-valued prefill)
- [x] Integration tests written (test full flow, not just units): KanbanView tests mount the full view with the API mocked; the server side of the drop is covered by the TKT-65LVAK handler tests
- [x] Happy path implemented: board columns from the anchor's offered statuses, drag re-points, Add in a column creates a task with that status
- [x] Edge cases from planning handled: Other column, uncoloured columns without `style_from`, prefill that survives a template switch
- [x] Error handling in place (errors surfaced, not swallowed): a failed drop rolls back the optimistic update and shows a toast (existing moveCard mutation)

## Test Quality

- [x] Using fixture builders or factories for test data: small status and task fixtures and a `page()` helper in KanbanView.columnsFrom.test.ts
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: on the demo server a drag re-points the status and an open detail panel follows it
- [x] Each acceptance criterion verified with test scenario from planning: AC1 and AC3 by the column resolution in useRelationColumns and the demo boards (Klantportaal and ISO-audit offer different sets); AC2 and AC4 by the tests above
- [x] Edge cases manually verified: the demo seed has a task whose status Klantportaal does not offer ("Toegankelijkheid reviewen"); it is placed under Other by useRelationColumns

**Verification Evidence:**
- Demo: a drag on the board re-points `heeft_status` and the side panel follows the move.
- `go test ./internal/...` passes.
- Frontend vitest full suite passes: 3770 tests in 255 files. `vue-tsc` is clean.
- `golangci-lint` reports 0 issues on the changed packages.
- Postgres tests (pgstore, entitymanager with `-tags postgres`) pass against a throwaway database.

## Quality

- [x] Code follows project patterns (check similar code): config follows the enum kanban validation; the shared colour table in styleColors.ts is used by Badge.vue and the board
- [x] Checked for DRY opportunities: R2-11 removed a duplicated colour table; column resolution is shared with the grouped list (TKT-JO8PN3)
- [x] No security issues introduced: no new endpoint; writes go through the relations PATCH
- [x] No silent failures (errors logged AND returned): validation errors name the kanban and the key
- [x] No debug code left behind
