---
id: IMPL-KC18SB
type: implementation-checklist
title: 'Implementation: Version snapshots do not record which face they captured'
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

- `e2e/tests/faces-history.spec.ts` flipped from fixme and passing on
postgres: restoring v1 of `POL-001@draft` restores the draft only, the published
face keeps its later title. The other 41 faces and history specs pass alongside
it.
- `internal/dataentry/history_face_test.go`: timeline, snapshot, restore onto
a live face, recreate of a deleted face at its own id, and ACL on `type@face`
(403 for a read-only face, 404 for a hidden one).
- `internal/cli/history_address_test.go`: bare faced id refused with the faces
named (live and deleted), explicit face and unfaced ids pass.
- `storetest.RunVersionTests` on sqlite and postgres: snapshots record their
face; an entity purge and a relation purge stay inside one face or tail.
- Manual smoke with the sqlite build on an unfaced project: `rela history`
and `rela history-purge` on a bare unfaced id behave as before.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
