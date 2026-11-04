---
id: IMPL-4JRXDZR
type: implementation-checklist
title: 'Implementation: E2E flakes: contract test SSE windows and form page helpers'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: the change is to tests only)
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: existing fixture project; one type added)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Contract test: opening the stream before the reset fails the old test
with seed 516079759 and passes the fixed one. Loop on the final code: see the
review checklist for counts.
- forms.spec.ts:45: dry-run responses delayed 1.5s fail the old test 3/3 and
pass the fixed one 3/3.
- forms.spec.ts and wizard.spec.ts, 12 workers, repeat 10: 370 passed;
after the review fixes, repeat 5 with the contract loop running: 185 passed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
