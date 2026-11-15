---
id: IMPL-1CFZLJ1
type: implementation-checklist
title: 'Implementation: Kanban quick-add'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestValidateConfig_KanbanQuickAdd table test; KanbanView.quickAdd.test.ts, 9 cases)
- [x] Integration tests written (test full flow, not just units) (TestCreate_QuickAddShapeAppliesDefaultTemplate runs the create through Manager; e2e kanban-quick-add.spec.ts creates through the real server and reloads)
- [x] Happy path implemented (title + column value, and swimlane value in a cell)
- [x] Edge cases from planning handled (columns_from link, Other column with no link, blank title, IME Enter, no create permission, quick_add off)
- [x] Error handling in place (errors surfaced, not swallowed) (a refused create shows under the input and the text stays; a link failure shows a toast)

## Test Quality

- [x] Using fixture builders or factories for test data (quickAddMetamodel extends columnsFromMetamodel; seedSchema/seedBoard/mountBoard in Vitest)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via Playwright against the built server)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./internal/dataentryconfig/ ./internal/entitymanager/ ./internal/configedit/`: ok.
- `npx vitest run src/views/KanbanView.quickAdd.test.ts`: 9 passed. `KanbanView.test.ts`: 26 passed.
- `npx vue-tsc -b`: clean.
- e2e `kanban-quick-add.spec.ts`: 3 passed. `kanban.spec.ts`, `kanban-collapse.spec.ts`, `kanban-wall.spec.ts`, `kanban-summary.spec.ts`: 25 passed.
- golangci-lint on the three Go packages: clean after one lll fix. `just arch-lint`: OK. eslint on changed files: no errors.

## Quality

- [x] Code follows project patterns (check similar code) (validator follows validateCalDAVConstructible; slots follow existing board slots)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (reuses plural, quoteAll and
sortedPropertyNames from validate_caldav.go)
- [x] No security issues introduced (writes go through POST create and entitymanager.Manager; the control follows _actions.create)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
