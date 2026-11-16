---
id: IMPL-AHRYEUC
type: implementation-checklist
title: 'Implementation: One step at a time over a set of entities'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestValidatePages walk cases, TestPosition_ListIDAppliesCondition, TestScopeFromParam_ListIDNeedsSourceList, PageView.test.ts "walking a list step")
- [x] Integration tests written (test full flow, not just units) (e2e pages.spec.ts: walking step and row not in the list)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty list, ?item= outside the list, failed read, PageDown while loading, world kept in links)
- [x] Error handling in place (errors surfaced, not swallowed) (error and missing states in PageWalk)

## Test Quality

- [x] Using fixture builders or factories for test data (seedEntity, installPagesConfig, e2e SEED)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (Playwright run against the e2e server, screenshot reviewed)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `npx playwright test tests/pages.spec.ts`: 11 passed.
`npx vitest run src/views/PageView.test.ts`: 41 passed. `go test
./internal/dataentry/ ./internal/dataentryconfig/ ./internal/conditionlint/`:
ok. `npx vue-tsc -b` clean. The screenshot shows "1 of 4", Previous row
disabled, Next row, and the feature's detail view under the step bar.

## Quality

- [x] Code follows project patterns (check similar code) (useNavStatus refetch pattern, listBaseParams as the one list-read builder)
- [x] Checked for DRY opportunities (walkScope derives the position scope from listBaseParams)
- [x] No security issues introduced (rows and position from the gated list pipeline; list_id only narrows)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
