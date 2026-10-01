---
id: IMPL-SMQ54L
type: implementation-checklist
title: 'Implementation: View table cells render every typed value as a badge'
started: "2026-09-30"
completed: "2026-09-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (EntityDetail.table.test.ts)
- [x] ~~Integration tests written~~ (N/A: frontend-only rendering change; the mount test drives the real component with a server-shaped view response)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty cell stays blank; relation column keeps the joined string; multiselect takes the list)
- [x] ~~Error handling in place~~ (N/A: pure rendering, no failure path)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: one small view() builder local to the file)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object~~ (N/A: no property comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Atlas demo on postgres, project PROJ-30ZY "Taken"
table: titles render as plain links, due dates as "26 sep 2026", status keeps
its colored badge. The regression test fails against develop's EntityDetail.vue
and passes with the fix.

## Quality

- [x] Code follows project patterns (mirrors nestedCellsFor and EntityList's per-cell cache)
- [x] Checked for DRY opportunities (four duplicated Badge blocks replaced by one helper)
- [x] No security issues introduced
- [x] ~~No silent failures~~ (N/A: no error path)
- [x] No debug code left behind
