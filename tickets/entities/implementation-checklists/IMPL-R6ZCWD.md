---
id: IMPL-R6ZCWD
type: implementation-checklist
title: 'Implementation: Writes resolve their face differently from reads, so a create always lands on the bare row'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

New regression coverage in `internal/entitymanager/facedwrite_test.go`: the
authorized face and the written face agree; a denied face is refused; a faced
type refuses a create naming no face and a faceless type refuses one naming a
face (symmetric); `UpdateEntity` reads its pre-image at the authorized face;
`unique:` is enforced within a face and two faces of one entity do not collide;
`ApplyEntity` probes the face the body names; two entities of a faced type get
distinct ids.

Store-level conformance (`internal/store/storetest/states.go`, so all four
backends): a named face needs no zero-coordinate row; deleting the zero
coordinate leaves siblings reachable; `HighestID` sees an entity that exists
only at a named face.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`conceptOnlyACL` records the faces it was ASKED about rather than only
returning a verdict, because a test that checks the verdict alone cannot
distinguish authorizing the right row from authorizing the wrong one that
happened to agree. That is the property this bug was about.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Reproduced the defect before fixing, twice: an explicit `Face: "concept"`
produced a row at `face=""`, and under a face-scoped ACL the gate was asked
about `concept`, answered allow, and the row landed on the face that principal
is denied. Both now pass as regression tests.

Ran the postgres suite against a real database
(`RELA_TEST_DATABASE_URL=postgres:///rela_hc6i2t_test`), not just the skipped
DB-gated form. That is what caught the delete/create race
(`TestDeleteEntity_RacingStateCreateLeavesNoHeadlessFace`) and validated the
reshaped `unique:` index and the new family advisory lock.

`just check` exits 0 (lint, arch-lint, plimsoll, comment-lint, lint-md, full
test suite). `just coverage-check` passes at 79.6%. `just docs-check` clean.
Frontend: `vue-tsc` clean, 2446 tests pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The family-row lookup recurs in three packages with three different store
interfaces, so it is three small helpers (`anyFaceOf`, `readableFaceOf`,
`seedRowOf`) rather than one: `readableFaceOf` additionally applies the read
gate while choosing, which the other two must not do. Collapsing them would
have meant a parameter deciding whether to enforce ACL, which is the shape
worth avoiding on a gate.

A security review of the diff found four issues; all four are fixed and
verified (ID collision, the `unique:` postgres backstop, the delete/create lock
mismatch, and `ApplyEntity`'s missing face check). The residual race is
BUG-22XSH3.
