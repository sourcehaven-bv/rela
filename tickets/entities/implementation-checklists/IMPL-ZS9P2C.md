---
id: IMPL-ZS9P2C
type: implementation-checklist
title: 'Implementation: Bounded relation replace skipped the audit log on a type mismatch'
started: "2026-10-22"
completed: "2026-10-22"
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

- `TestPatchRelations_DisallowedTypeOnBoundedRelationAudited` drives the PATCH
handler with a principal on ctx. On develop it fails: the audit holds only
`create-relation>C-1` (full linkage) and only `delete-relation>C-1` (re-point).
With the fix both cases audit every create and remove with the principal, and
still return 200 with `target_type_not_allowed`.
- `TestPatchRelations_DisallowedTypeOverBoundIs422`: a disallowed create over
`max_outgoing` is still 422 `cardinality_exceeded`, nothing written or audited.
- `TestReplaceRelations_TolerateTypeMismatch` (entitymanager): tolerated create
re-points and is audited with subject and principal; still counts against the
bound; without the flag the replace fails and changes nothing.
- `TestPatchRelations_DisallowedTypeWithDeniedRemoveWritesNothing` still passes
(403, nothing written).
- `go test -race ./internal/dataentry/... ./internal/entitymanager/...` green.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
