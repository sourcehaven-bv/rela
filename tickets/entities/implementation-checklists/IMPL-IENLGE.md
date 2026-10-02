---
id: IMPL-IENLGE
type: implementation-checklist
title: 'Implementation: Authored-span property rows overlap and do not edit inline'
started: "2026-10-01"
completed: "2026-10-01"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CSS-only layout change; jsdom has no layout engine, so geometry is covered by the e2e test)
- [x] Integration tests written (test full flow, not just units) (e2e `inline-edit values keep room inside narrow span cells`, fixture section "Narrow fields")
- [x] Happy path implemented
- [x] Edge cases from planning handled (span 2 narrower than the label; span 4 with badges; `stacked` rows; inline control min-width capped at the cell)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error paths in a CSS change)

## Test Quality

- [x] Using fixture builders or factories for test data (SEED task and the shared fixture project)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: geometry assertions only)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** rela-server-postgres on a copy of atlas-dev with the
latest atlas config, TASK-N4W5. At 1100px span-4 and span-2 fields stack label
above value; nothing overlaps and Status is visible. At 1440px span-4 fields
stay side by side and Status stacks. With `render: input` on the section the
Slaagkans picker opens. The new e2e test fails on the old RlDetailField (status:
no width, runs past its cell, overflows) and passes with the fix. Frontend unit
tests 3626 passed, library tests 444 passed, library `npm run check` passed,
typecheck clean, lint 0 errors.

## Quality

- [x] Code follows project patterns (check similar code) (fix lives in the library component, per rela-components owning layout)
- [x] ~~Checked for DRY opportunities~~ (N/A: two CSS declarations)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: CSS-only)
- [x] No debug code left behind
