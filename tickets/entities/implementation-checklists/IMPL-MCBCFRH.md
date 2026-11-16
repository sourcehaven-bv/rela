---
id: IMPL-MCBCFRH
type: implementation-checklist
title: 'Implementation: Create a column from the board'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestValidateConfig_KanbanAddColumn; addEnumColumn.test.ts, 8 cases; KanbanView.addColumn.test.ts, 8 cases)
- [x] Integration tests written (test full flow, not just units) (e2e kanban-add-column.spec.ts saves through the real /_configure/save and checks schema.yaml and data-entry.yaml)
- [x] Happy path implemented (value, label and board column appended in one Configure save, then reload)
- [x] Edge cases from planning handled (label equal to value, board without columns:, accents and symbols, duplicate value or label)
- [x] Error handling in place (errors surfaced, not swallowed) (refusals, 422 problems and 409 conflicts show inline; the text stays)

## Test Quality

- [x] Using fixture builders or factories for test data (snapshot() builder; addColumnMetamodel extends columnsFromMetamodel)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via Playwright against the built server with config editing on)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `go test ./internal/dataentryconfig/`: ok. golangci-lint on the package: 0 issues.
- `npx vitest run src/views/KanbanView src/configure/addEnumColumn.test.ts packages/rela-components/src/components/board`: 170 passed.
- `npx vue-tsc -b`: clean. eslint on changed files: no errors.
- e2e `kanban-add-column.spec.ts`: 3 passed. `kanban-quick-add`, `kanban-wall`, `kanban.spec.ts`: 24 passed.

## Quality

- [x] Code follows project patterns (check similar code) (save flow follows configure/useReview.ts; tree edits use configure/tree.ts)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (reuses machineName and the tree helpers)
- [x] No security issues introduced (no new config writer; /_configure/save enforces config:edit and the allowlist)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
