---
id: IMPL-6PIFBQ
type: implementation-checklist
title: 'Implementation: Enable gosec G702 (command injection) with reviewed exec-site annotations'
status: done
---

## Development

- [x] Unit tests written for new code — `commands_test.go` updated
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled — `renderCommand` now rejects an unsafe
`entryID` at the point of use rather than trusting the caller
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

Confirmed the rule is genuinely enforcing rather than silently still excluded:
removing one `#nosec` makes the finding reappear, and restoring it clears it. A
suppression that does nothing would otherwise be indistinguishable from a
working one.

Two darwin test cases in `commands_test.go` pinned exact argv and were updated
for the added `--`.

Static checks: `golangci-lint run ./...` with G702 enabled reports 0 issues;
`go build ./...` and the full `go test ./...` pass, re-verified after rebasing
onto current `develop`.
