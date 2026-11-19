---
id: IMPL-TXDKOTA
type: implementation-checklist
title: 'Implementation: Open step scrolls the app shell frame and hides the top bar'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (appShellFrame.test.ts; fails on develop)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: CSS only; the test runner has no layout, so it was checked in a browser)
- [x] Happy path implemented
- [x] Edge cases from planning handled (sticky mobile top bar: scroll margin on the run card)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: CSS only)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: source test)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] Feature manually tested end-to-end (a browser page with the shell layout: with hidden, scrollIntoView moves the frame; with clip it does not)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** appShellFrame.test.ts fails on develop (frame
overflow: hidden) and passes with clip. In a browser page with the shell layout,
scrollIntoView moved a hidden frame and left a clip frame at 0.
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (source test like focusRing.test.ts)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (two CSS declarations; nothing to share)
- [x] No security issues introduced (CSS only)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: CSS only)
- [x] No debug code left behind
