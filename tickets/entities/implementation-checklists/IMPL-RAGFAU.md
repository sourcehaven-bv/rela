---
id: IMPL-RAGFAU
type: implementation-checklist
title: 'Implementation: Collapsible kanban columns'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (useKanbanCollapse.test.ts, KanbanView.collapse.test.ts, TestKanbanColumnCollapsed_YAMLAndJSON)
- [x] Integration tests written (e2e/tests/kanban-collapse.spec.ts against the real server: plain board, swimlane board, reload, config default)
- [x] Happy path implemented
- [x] Edge cases from planning handled (stale stored column ignored, all-collapsed board, empty collapsed column shows 0, corrupt storage)
- [x] Error handling in place (corrupt localStorage falls back to config defaults; a failed setItem still folds the column for the visit)

## Test Quality

- [x] Using fixture builders or factories for test data (ticket() helper; e2e fixture board bug-lanes)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Screenshots of the e2e server (bug-board open,
bug-board with New collapsed, bug-lanes with In Progress collapsed by config):
the collapsed column is a rail with vertical title and count (plain) or a
compact header pill with count (swimlane), matching the Atlas TASK-8997C mockup.
Reload keeps the state (e2e). kanban.spec.ts (18 tests) still passes; full
frontend vitest 3864 passed; rela-components vitest 456 passed and `npm run
check` passes; go test dataentryconfig + dataentry pass.

## Quality

- [x] Code follows project patterns (check similar code) (localStorage persistence mirrors useListGrouping; library emits, caller owns state, as with expandSection)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (client-side view state only; counts come from the gated board read)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
