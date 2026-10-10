---
id: IMPL-CL09VD
type: implementation-checklist
title: 'Implementation: dataentry 500 responses send the internal error text to the client'
started: "2026-10-09"
completed: "2026-10-09"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: an injected store failure is not reachable by hand; the handler tests drive the real handlers)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: same reason; the guard was verified by planting two leak shapes and watching it fail)

**Verification Evidence:** TestNoInternalErrorDetail,
TestWriteInternalError_HidesTheCause and TestCloneEntity_UniqueCollisionIs422
pass; go test ./internal/dataentry passes; golangci-lint 0 issues. A planted
`detail := fmt.Sprint(gerr)` 500 and a planted `ganttError{500, ...,
gerr.Error()}` both failed the guard.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
