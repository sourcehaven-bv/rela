---
id: IMPL-1O8SGCV
type: implementation-checklist
title: 'Implementation: On mobile the hamburger overlaps the next-action banner'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: a CSS margin; jsdom has no layout, so the e2e test measures it)
- [x] Integration tests written (test full flow, not just units) (e2e/tests/mobile-layout.spec.ts 'the hamburger keeps clear of the next-action card')
- [x] Happy path implemented
- [x] Edge cases from planning handled (pages with a sticky topbar keep margin-bottom -44px, which outranks the new margin)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new failure path)

## Test Quality

- [x] Using fixture builders or factories for test data (the test extends the testProject fixture with a banner next action)
- [x] No hardcoded values in assertions when object is in scope (the 8px minimum gap is the requirement itself)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] Feature manually tested end-to-end (the mobile e2e project runs the real server and browser)
- [x] Each acceptance criterion verified with test scenario from planning (the gap is at least 8px; all mobile-layout tests pass)
- [x] ~~Edge cases manually verified~~ (N/A: covered by the existing mobile-layout tests for pages with a topbar and for scrolling)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (uses the --rl-space-2 token plus the sticky top)
- [x] ~~Checked for DRY opportunities — repeated literals, expressions, or~~ (N/A: one declaration)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (CSS only)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no new failure path)
- [x] No debug code left behind
