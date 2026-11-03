---
id: IMPL-DBA678
type: implementation-checklist
title: 'Implementation: Create menu puts the current page''s type first'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
  - `frontend/src/composables/useViewEntityTypes.test.ts`: `viewEntityTypes` per view kind (list, kanban, calendar with duplicate sources, gantt, unknown id, dashboard, document); `pageTabView` (plain page, entity page, two-segment route, tab without target, unknown tab or page); `leadWith`.
  - `frontend/src/utils/pageContext.test.ts`: `routeTabId`.
- [x] Integration tests written (test full flow, not just units)
  - `SpaceCreateMenu.test.ts` 'SpaceCreateMenu order': mounted menu with route and stores for list, kanban, calendar, page tab switch, entity page (narrowing then ordering), type not in `create:`, separator only when rows follow.
  - e2e `e2e/tests/spaces.spec.ts`: `create: [task, bug]`; list/tasks gives Task, Bug; list/bugs and kanban/bug-board give Bug, Task with one separator; list/features gives config order, no separator.
- [x] Happy path implemented
- [x] Edge cases from planning handled (unknown view id, schema not loaded, view type not offered, duplicate calendar sources, two-segment entity page route all give config order or the deduplicated types)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: pure ordering; no failure path)

## Test Quality

- [x] Using fixture builders or factories for test data (typed config fixtures per file)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- e2e against `just build-server-e2e`: spaces.spec (8 tests) and space-create-page-link.spec pass. A screenshot of `/s/support/kanban/bug-board` showed the menu as Bug, separator, Task.
- Vitest full run: 305 files, 4289 tests passed (before review fixes); affected files pass after.
- `vue-tsc -b` clean; eslint adds no warnings.

## Quality

- [x] Code follows project patterns (check similar code): `RlMenuSeparator` as in AccountMenu/EntityActionsMenu; open-tab rule shared with PageView through `routeTabId`.
- [x] Checked for DRY opportunities: `routeTabId` replaces the copy of PageView's tab rule.
- [x] No security issues introduced (order only; the offered set is unchanged)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A)
- [x] No debug code left behind
