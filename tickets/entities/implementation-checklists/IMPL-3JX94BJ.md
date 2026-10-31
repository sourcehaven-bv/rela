---
id: IMPL-3JX94BJ
type: implementation-checklist
title: 'Implementation: Autosave misjudges its own state across saves of one tab'
started: "2026-10-31"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (five unit tests in useAutoSave.conflicts.test.ts, one per case)
- [x] Integration tests written (test full flow, not just units) (the two-tab property test exercises every fix against the fake API)
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: the orders need two tabs and exact answer timing; covered by the scheduler-driven property test)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: same reason; each edge case is a fault or command in the property test)

**Verification Evidence:**

- Each unit test fails with its fix reverted (mutations M1, M3b, M4 and the relations base).
- The two-tab property passes 2000 runs; it failed on each case before the fix.
- Full suite: 365 files, 5480 tests pass; vue-tsc clean; eslint 0 errors.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
