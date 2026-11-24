---
id: IMPL-PUBUTW7
type: implementation-checklist
title: 'Implementation: List assign: defaults for create, drop and move'
started: "2026-11-24"
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

**Verification Evidence:**
rela-server on a copy of examples/todo-app: GET `_assign?list=today` answered
`{when: today}`; a missing page target answered 404; POST to today with a
duplicate and a missing id wrote TD-IN01 once (`matches: true`) and reported
TD-NOPE as 404 `not_found`; a page drop related TD-IN02 to PRJ-10K, and TD-IN01
appeared in the Today list. Drag and drop in the SPA is covered by unit tests
(drop target parsing, typecheck) and was not exercised in a browser.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
