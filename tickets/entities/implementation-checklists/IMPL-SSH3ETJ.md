---
id: IMPL-SSH3ETJ
type: implementation-checklist
title: 'Implementation: POST /api/v1/_batch endpoint, OpenAPI and API reference'
started: "2026-11-04"
completed: "2026-11-04"
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

End-to-end verification runs through the HTTP handler tests, which drive the
real mux, ACL and store.

- `go test -race ./internal/dataentry/` passes (360 s run), including all `TestV1Batch_*` tests.
- HTTP tests cover: 501 on fs, 405, cross-origin refusal, 413 over 1 MiB, 400 over 100 operations, shape errors with JSON pointers, field-gate and affordance denials (403 with operation, rule_kind, rule_id), type-disallowed edges, hidden end and hidden source indistinguishable from missing, redaction of an unreadable create.
- sqlite build: `TestV1Batch_SQLite_Creates` and `ConflictRollsBack` (409, nothing written).
- OpenAPI: `TestBatchPath` and the openapi golden.
- `just docs` regenerates docs/acl-security.md with no diff.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
