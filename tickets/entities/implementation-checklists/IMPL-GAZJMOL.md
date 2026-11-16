---
id: IMPL-GAZJMOL
type: implementation-checklist
title: 'Implementation: Mutable record types'
started: "2026-11-16"
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

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by HTTP handler tests over the real service and file backend; no manual server run)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** `go test ./internal/records/... ./internal/metamodel/
./internal/audit/ ./internal/appbuild/` and `-tags sqlite ./internal/sqlitedb/
./internal/records/sqliterecords/` pass; TestRecordsHandler_ChangeRow covers
200, 204, 400, 404, 405 and 409 over HTTP. `go build -tags postgres ./...` and
`-tags sqlite ./...` succeed; arch-lint is clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
