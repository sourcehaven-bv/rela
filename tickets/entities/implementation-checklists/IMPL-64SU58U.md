---
id: IMPL-64SU58U
type: implementation-checklist
title: 'Implementation: TestHiddenProperty_NotCompared fails when a random entity id contains 9'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

## Development

- [x] ~~Unit tests written for new code~~ (N/A: test-only change; the fix is to an existing test)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: test-only change)
- [x] Happy path implemented
- [x] Edge cases from planning handled (TestHiddenProperty_RealPolicy uses the same helper)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no production code)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: existing world helpers reused)
- [x] No hardcoded values in assertions when object is in scope (assertions use hiddenWeight)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test-only change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Old test: 20 failures in `go test -count=200 -run
TestHiddenProperty_NotCompared ./internal/mapimport/`. New test: 0 failures in
200 runs of both hidden-property tests; full package passes; golangci-lint
reports 0 issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (two identical leak checks moved into assertHidden)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: test-only change)
- [x] No debug code left behind
