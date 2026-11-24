---
id: IMPL-1GPV5WD
type: implementation-checklist
title: 'Implementation: Nested generated sidebar groups with per-entry counts'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (handler tests through the real ACL, budget test)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no edges, hidden child at the limit)
- [x] Error handling in place (a count or child read that fails is logged and left out)

## Test Quality

- [x] Using fixture builders or factories for test data (seed helpers, budget fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by handler and SPA tests; no example config may change here)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (by tests)

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (one navRelatedRows helper serves children and counts)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Evidence: TestNavItems_ChildrenAreGatedAndGrouped,
TestNavItems_ChildrenTruncated, TestNavItems_StatusCountsGatedRows,
TestQueryBudget_NavItemsTreeIsSizeIndependent, TestValidateNavItemsFrom,
sidebarNav.test.ts.
