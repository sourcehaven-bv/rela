---
id: IMPL-RCEP69V
type: implementation-checklist
title: 'Implementation: View section tables on phones, list band and title link colour'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (sectionTable.test.ts, SectionTableRowCells.test.ts, EditableRowCells.test.ts, SectionTableCellContent.test.ts, EntityList.titleLink.test.ts, TestViewColumnsToV1_Primary, TestValidateConfig_PrimaryColumnPlacement)
- [x] Integration tests written (test full flow, not just units) (e2e mobile-layout.spec.ts stacks a seeded view section and measures the list header gap)
- [x] Happy path implemented
- [x] Edge cases from planning handled (null cell values, editable empty cells, reorder handle and drop line, grouped tables, empty link cells)
- [x] Error handling in place (errors surfaced, not swallowed) (primary on an unsupported surface is a config error)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing mount helpers and e2e SEED)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
A local server built from the branch on a copy of tickets/: the FEAT-M448 Bug Fixes section stacks at phone width; all_tickets sits flush under the header with accent titles on desktop and phone. The e2e mobile-layout and table-inline-edit specs pass (17 passed, 1 skipped). Focused vitest, vue-tsc, eslint and the Go tests pass. Not checked against the saas-dev harbourmaster env after the fix.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors the dashboard card stacked layout from BUG-ST02A0S)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (sectionTable.ts serves the plain and the editable row)
- [x] No security issues introduced (layout and a config flag only)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
