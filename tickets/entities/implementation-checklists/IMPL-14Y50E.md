---
id: IMPL-14Y50E
type: implementation-checklist
title: 'Implementation: Pin the write-ACL refusal of relations a conversion removes'
started: "2026-10-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] ~~Happy path implemented~~ (N/A: test only; the code exists since 563989583)
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no production code)

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
`go test -race ./internal/entitymanager/...` passes. Mutation check: with the
`authorizeRemovals` refusal disabled in `conversion_flow.go`, the read-only
case fails with "Convert = <nil>, want acl.ErrForbidden". golangci-lint: 0
issues; `just arch-lint`: OK.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
