---
id: IMPL-6B4V6VR
type: implementation-checklist
title: 'Implementation: Flow tasks as a next-action source'
started: "2026-11-18"
completed: "2026-11-19"
status: done
---

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

**Verification Evidence:** e2e/tests/flow-task-hint.spec.ts drives the real
server and SPA: an hr user starts the onboarding flow on a person, the list page
shows the banner "Complete Collect documents for Ann Smith", Open lands on
/entity/person/PER-1?run=... with the run card focused and in view, completing
the step clears the banner, and snoozing hides the banner while the step form
stays in the Flows panel. Unit tests cover no identity, no flows, unread
subject, flow filter, oldest pick, snooze, new state, cooldown, cap, link
escaping and the read budget.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
