---
id: IMPL-YIRSS83
type: implementation-checklist
title: 'Implementation: Flow run panel for people who only do a step'
started: "2026-11-18"
completed: "2026-11-18"
status: done
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

**Verification Evidence:** Ran a copy of the harbourmaster project on develop
and on this branch, as a customer and as staff, in light and dark mode at 1400
and 800 px. The customer sees a vertical form, labelled fields, no internals and
no cancel. Staff see everything with humanised names. Start is hidden while the
signup run is unfinished. The Playwright flows spec covers an applicant and an
HR user.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
