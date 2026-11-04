---
id: IMPL-2GNU0J1
type: implementation-checklist
title: 'Implementation: Atomic batch engine: entitymanager.CreateBatch'
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

- `go test -race ./internal/entitymanager/` passes, including every `TestCreateBatch_*` case.
- Postgres: `RELA_TEST_DATABASE_URL=... go test -tags postgres -run TestCreateBatch ./internal/entitymanager/` passes; the rollback-only subtests ran.
- sqlite: `go test -tags sqlite ./internal/dataentry/ -run Batch` passes; `ConflictRollsBack` shows a failing batch leaves no rows.
- fs/memory: a failure after rows were written returns `BatchKeptError`, and the kept rows are audited (`TestV1Batch_KeptRowsAre500WithoutContent`).
- A type-disallowed edge and a missing end fail the whole batch with no rows written.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
