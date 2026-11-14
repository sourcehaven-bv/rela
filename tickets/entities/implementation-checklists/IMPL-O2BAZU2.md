---
id: IMPL-O2BAZU2
type: implementation-checklist
title: 'Implementation: Budget tests count rows read, not only store calls'
started: "2026-11-13"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: test harness; a row with an error is not charged and the error passes through)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test-only change; the tests are the verification)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
`go test` on every touched package passes. `TestAssertScaleFree_FullScanFails`
shows one full-scan call fails on rows only. Converting the budget tests
found real scans; those tests stay calls-only with `TODO(TKT-FED6FMM)`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] ~~No security issues introduced~~ (N/A: test-only code)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: test harness)
- [x] No debug code left behind
