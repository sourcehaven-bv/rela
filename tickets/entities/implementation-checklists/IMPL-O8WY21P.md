---
id: IMPL-O8WY21P
type: implementation-checklist
title: 'Implementation: done_when per step'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestPageDoneEntries, TestCompileViewConditions_PageDone, page_status_test.go, PageView.test.ts "done marks")
- [x] Integration tests written (test full flow, not just units) (TestPageStatus_JudgesTheWorldsRows through the router; e2e done check)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty list is not done, hidden rows, hidden anchor 404, gated tab omitted, uncompiled condition skipped, world)
- [x] Error handling in place (errors surfaced, not swallowed) (a step that cannot be judged is logged and omitted)

## Test Quality

- [x] Using fixture builders or factories for test data (seedEntity, stepsPage, seedFacedTickets)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (Playwright: Bugs step checked, Each feature not, Next enabled)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `npx playwright test tests/pages.spec.ts`: 11 passed.
`TestQueryBudget_PageStatusIsSizeIndependent` pins 7 and 8 store calls.
`TestPageStatus_HiddenRowsDoNotCount` and `TestPageStatus_EntityPage` pin the
ACL behaviour. `rela validate` on the e2e project caught a bad `!=` and pointed
to `~=`.

## Quality

- [x] Code follows project patterns (check similar code) (nav status: configuredListRows, primeVerdicts, applyViewCondition)
- [x] Checked for DRY opportunities (navStatusRows and pageStepDone share configuredListRows)
- [x] No security issues introduced (judged over gated rows; done state off the sidebar; world guard scans the handler)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
