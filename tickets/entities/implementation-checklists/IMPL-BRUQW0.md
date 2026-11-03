---
id: IMPL-BRUQW0
type: implementation-checklist
title: 'Implementation: rela.md.parse drops the closing line of multi-line HTML blocks'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error paths)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: inputs are literal markdown strings)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: output is compared with the input string itself)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `TestMdRoundTripHTMLComment` (single line, multi
line, closer on its own line, blank line and list line inside the comment)
failed on 3 of 4 cases before the fix and passes after it; `go test
./internal/lua/` passes. End to end: the atlas recurrence test
(`scripts/tests/test-recurrence.py`, atlas branch `feat/recur-taakinhoud`) run
with this branch's `rela` binary leaves every template comment of a spawned task
closed; with the released binary it does not.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
