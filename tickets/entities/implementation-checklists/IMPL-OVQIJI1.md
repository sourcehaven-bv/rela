---
id: IMPL-OVQIJI1
type: implementation-checklist
title: 'Implementation: Computed group keys for data-entry lists'
started: "2026-11-24"
status: done
completed: "2026-11-24"
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
Ran rela-server on examples/todo-app (merged with #2575 locally): list_id=today
stamps day/evening/deadline as configured; completing TD-UP07 (repeat 1w)
created the next to-do at 2026-12-05; reopening TD-LB01 removed completed_on;
creating a scheduled to-do without start_date returns 422. Go handler, ACL,
budget and validation tests plus vitest pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
