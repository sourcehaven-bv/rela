---
id: IMPL-G3KH03
type: implementation-checklist
title: 'Implementation: TestConvert_RefusesToCopyAnUnreadableField/no_value flakes: map order runs Convert before Preview'
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: test-only fix; the changed subtest is the regression test)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: test-only fix)
- [x] Happy path implemented
- [x] Edge cases from planning handled (both subtests: hidden value refuses both calls, no value allows both)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no production code changed)

## Test Quality

- [x] Using fixture builders or factories for test data (existing newConversionFixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test-only fix)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

`go test -race -run TestConvert_RefusesToCopyAnUnreadableField -count=500
./internal/entitymanager`: 64 of 500 runs fail on develop, 0 of 500 with the
fix.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no production code changed)
- [x] No debug code left behind
