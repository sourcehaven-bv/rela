---
id: IMPL-89JODT
type: implementation-checklist
title: 'Implementation: testifylint finding in sqlite-tagged test is not caught by CI lint'
started: "2026-10-03"
completed: "2026-10-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; one assertion changed)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no new code)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: no edge cases in a one-line assertion change)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no error paths touched)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: test data unchanged)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: assertion compares a boolean expression)
- [x] ~~Only specifying values that matter for the test~~ (N/A: test data unchanged)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolation)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons changed)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: bug has no planning acceptance criteria; verified via lint)
- [x] ~~Edge cases manually verified~~ (N/A: no edge cases)

**Verification Evidence:** `golangci-lint run --build-tags sqlite
./internal/store/sqlitestore/` reports 0 issues (was 1 testifylint). `go test
-tags sqlite -run Naive ./internal/store/sqlitestore/` passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
