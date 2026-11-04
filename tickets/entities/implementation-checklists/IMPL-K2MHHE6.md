---
id: IMPL-K2MHHE6
type: implementation-checklist
title: 'Implementation: AtomicWrites capability flag and capabilities.atomic_batch on _schema'
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

- `/api/v1/_schema` carries `capabilities.atomic_batch`: false on the default fs build (`TestV1Batch_UnsupportedStoreIs501`), true on sqlite (`TestV1Batch_SQLite_CapabilityIsAdvertised`).
- The value comes from `store.TxRollsBack`, read through the innermost store.
- Golden schema files for decl-header and decl-jwt re-recorded; `TestGoldenParity` passes.
- Frontend type `capabilities?` added to `frontend/src/types/schema.ts`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
