---
id: IMPL-WPTRLOD
type: implementation-checklist
title: 'Implementation: List buckets: relative_weeks with this week and next week'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no e2e test added; groupRows and config validation tests cover the flow)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: bucket tests assert fixed calendar dates, as the existing ones do)
- [x] ~~Only specifying values that matter for the test~~ (N/A: bucket tests assert fixed calendar dates, as the existing ones do)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values in these tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: tests compare section ids and titles)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: the Sunday case is covered by unit tests only, not checked in the browser)

**Verification Evidence:** Go and frontend tests pass; checked in the browser
against a prototype project with nine tickets from overdue to later: sections,
order and counts correct.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
