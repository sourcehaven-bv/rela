---
id: IMPL-TEMOY32
type: implementation-checklist
title: 'Implementation: List title link colour without link:, nested and card sections on phones'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (EntityList.titleLink.test.ts, with and without link)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: CSS only; no e2e fixture has a nested section)
- [x] Happy path implemented
- [x] Edge cases from planning handled (long titles, levels without columns, inaccessible titles and rows without a target keep no link class)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error paths)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing list mount helper)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
A local server built from the branch on a copy of tickets/ with a nested section in the feature view: at 375px the values sit on their own line within the tree, at desktop width the row keeps one line. Focused vitest, vue-tsc and eslint pass.

## Quality

- [x] Code follows project patterns (check similar code) (the container query mirrors the table section one)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (CSS and a class only)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
