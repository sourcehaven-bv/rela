---
id: IMPL-V7K2NP4
type: implementation-checklist
title: 'Implementation: Session screen and confirms behind dialogs, 401 noise, tenant 404, action confirm colour'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (rlStacking.test.ts, ui.test.ts, EntityList.world.test.ts, EntityDetail.header.test.ts, EntityDetail.actions.test.ts, useToolbarActions.test.ts, tenant_status_test.go)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: the stacking test resolves the real stylesheets; the route is covered by the router walk)
- [x] Happy path implemented
- [x] Edge cases from planning handled (errors that are not refusals still show; non-GET on the status route answers 405 with Allow)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing mount helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
The production build declares --rl-z-modal:100 before 1000, both unlayered, so 1000 wins. The stacking test fails with stacking.css imported before tokens.css. Full vitest, vue-tsc and go test for dataentry and tenanthost pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (the status route returns a constant and sits behind the same auth as the other API routes)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
