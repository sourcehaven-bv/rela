---
id: IMPL-3ZTZ2B5
type: implementation-checklist
title: 'Implementation: tenanthost: a request racing the end of a lazy provisioning run starts a second run'
started: "2026-11-17"
completed: "2026-11-17"
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: reuses the package stubs newControl and stubProvisioner)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: concurrency fix in the host; covered by the deterministic HTTP-level test)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** TestProvision_LazyNoRerunAfterSuccess failed (calls =
2) before the fix and passes after. go test -race -count=1000 of it and
TestProvision_LazySingleFlight passes; the whole package passes with -race
-count=3; golangci-lint and comment-lint report 0 issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
