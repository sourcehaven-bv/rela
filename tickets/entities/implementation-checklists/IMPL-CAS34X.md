---
id: IMPL-CAS34X
type: implementation-checklist
title: 'Implementation: Optimistic concurrency at the store: expected-version on entity.Patch / UpdateEntity'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

`store.UpdateEntityIf` is implemented on all four backends, each keeping the
compare and the write inside ONE critical section — the property that makes it
a compare-and-swap rather than the check-then-write it replaces:

- **memstore**: both under one acquisition of `m.mu`.
- **fsstore**: under one acquisition of `s.mu`, comparing against the entity
freshly loaded from DISK. That is deliberate — a hand edit or a `git pull`
changes the file with no store write, and a token derived from the bytes
catches it where a per-write counter would not.
- **sqlitestore**: read and write inside one `BEGIN IMMEDIATE` transaction.
- **pgstore**: `SELECT ... FOR UPDATE` and the write in one transaction, so
the precondition holds across PROCESSES, which is the case the in-process
mutex never covered.

Errors are surfaced as a typed `*store.VersionConflictError` carrying
`Expected` and `Actual`, so a caller can retry with the observed version
instead of re-reading blind. It is `errors.Is`-able as `store.ErrConflict` and
recoverable with `errors.As` through the manager's wrapping.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The conformance suite asserts against the objects in scope (for example
`assert.Equal(t, read.GetString("title"), got.GetString("title"))` rather than
re-stating the literal), so a test cannot pass by agreeing with itself.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

The CAS suite runs inside `storetest.RunAll`, so every backend is held to it
rather than only the one the author had in mind.

*Negative control (the check that the suite can FAIL).* Independently
reproduced during this pass: sabotaging memstore's `updateEntityIf` so the
version comparison never runs (`if !cond.IsZero()` replaced by `if false`)
makes `TestConformance/CAS/StaleVersionConflicts` and
`TestConformance/CAS/RetryWithActualSucceeds` fail. Restoring the comparison
returns the package to green. This matters because a conformance suite that
cannot fail would let a future backend ship a no-op CAS and still look
conformant.

*Faced-state regression.* `TestFacedIDWrite_AuthorizesTheFaceItWrites` and
`TestFacedIDWrite_ExplicitFaceGrantStillWorks` both pass — these are the tests
that caught RR-1GM1NB, where the PATCH migration passed a bare id and so
authorized and wrote the default face.

*Backends.* `go test -race` green for `internal/store/...`,
`internal/entitymanager/...` and `internal/dataentry/...`. The pgstore
cross-process acceptance test (`cas_crossprocess_test.go`) is DB-gated and was
not exercised here — see the review checklist for what that leaves unverified.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Each backend routes its unconditional `updateEntity` through the conditional
core with a zero `UpdateCondition`, so there is ONE update path per backend
rather than two that could drift — the shape that would otherwise let the
unconditional path quietly lose a fix applied only to the conditional one.

On security: RR-1GM1NB was a real defect found in this area and fixed here (the
PATCH handler now builds the fused state ref so the face reaches both the
authorization subject and the row written), and RR-X0TGM4 (type-blind version
hash) was fixed by folding the value's type into the token.
