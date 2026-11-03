---
id: IMPL-IBMJSQY
type: implementation-checklist
title: 'Implementation: Data API request without identity gets 500 instead of 401'
started: "2026-10-30"
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
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values in the new tests)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: tests compare status codes and error types only)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
rela-server built from this branch, run on a copy of the GRC demo project
with `--principal-header X-Remote-User`. GET /api/v1/maatregelen without the
header: 401 `unauthenticated`; with the header: 200. Without
`--principal-header`: 401 (was 500). `go test ./internal/dataentry/...`
passes; golden corpus regenerated (`decl-header/anonymous.golden` now 401).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
