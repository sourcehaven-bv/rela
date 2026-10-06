---
id: IMPL-929TMB
type: implementation-checklist
title: 'Implementation: Section create menu has no background since the move to rela-components'
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: CSS-only fix; the guard reads source because Vitest does not apply scoped styles)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no runtime code changed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: source scan, no test data)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built rela-server from develop and from this branch
and opened an entity detail page whose section create offers two types, on ports
18180/18181. Before: `.create-menu` computed background `rgba(0, 0, 0, 0)`, item
colour `rgb(0, 0, 0)`, table header visible through the menu. After: background
`rgb(255, 255, 255)` (light) and `rgb(22, 22, 25)` (dark), item colour follows
`--rl-color-text`, hover shows `rgb(244, 244, 243)`. Guard
`cssCustomProperties.test.ts` failed before the fix with 17 offenders in six
components and passes after.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no runtime code changed)
- [x] No debug code left behind
