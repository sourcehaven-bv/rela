---
id: IMPL-2JZXHL
type: implementation-checklist
title: 'Implementation: Drag-and-drop reorder of a list scoped to one entity through an orderable relation'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (Go: order_place_test, move_test, relation_position_test, relation_order_test, relation_order_section_test; vitest: relationOrder, useListReorder, useSectionReorder, KanbanView.reorder; Storybook play tests for RlTable Reorderable and RlBoard Reorder)
- [x] Integration tests written (e2e/tests/relation-order.spec.ts drives the list tab, board tab and detail section against a real rela-server, with pointer drags, keyboard steps and a reload)
- [x] Happy path implemented
- [x] Edge cases from planning handled (tied/missing order values densify; page-edge keyboard steps send `step`; reader sort, grouping, swimlanes and hidden `_order_out` disable reordering; non-sibling ref gets the uniform 404; read-only `_order_out` gets 403)
- [x] Error handling in place (a failed move shows a toast and the rows return to the stored order; a failed column change on the board skips the place write)

## Test Quality

- [x] Using fixture builders or factories for test data (newOrderedTabApp, seedOrderableFixture, moveFixture; e2e seeds through the api fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran rela-server (e2e build) with a feature page whose `contains` relation is
`orderable: outgoing`, with a list tab, a board tab and a `display: table`
section. Screenshots are in `.ignored/shots/`.

- List tab: rows show in edge order. The handle drag shows a drop line on the
target row, and the drop moves the row. ArrowDown on a focused handle moves the
row a place. The order survives a reload.
- Board tab: a card dragged over another card shows a line above or below it,
including below the last card, and the drop moves it within its column. The
order survives a reload.
- Detail page: the Parts table section shows handles, and ArrowUp moves a row.
The order survives a reload.
- Clicking a column sort removes the handles.
- The full e2e suite gave 438 passed and 3 failed (forms, wizard,
pending-indicators). Those 3 pass when run alone, and this change does not touch
them.

## Quality

- [x] Code follows project patterns (check similar code) (package functions instead of methods on Manager/writeHandler/viewsHandler per plimsoll; consumer-side interfaces; library imports by path)
- [x] Checked for DRY opportunities (closestEdge shared by table and board; planRowMove shared by list and section reorder; defaultSortParam replaces three default_sort sites)
- [x] No security issues introduced (move runs the relation meta-field writability gate; sibling refs go through the read gate; order hidden by `visible:` is not applied or exposed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
