---
id: IMPL-KJ2I832
type: implementation-checklist
title: 'Implementation: Scoped relations: within and on_change'
started: "2026-11-24"
completed: "2026-11-24"
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

**Verification Evidence:** Built `rela` from this branch and ran it on a scratch
project with todo, project, heading (owned by project) and `under_heading`
(within in_project, on_change clear). Linking a todo in PRJ-A to the heading of
PRJ-B was refused with "relation "under_heading" must point to an entity owned
by the target of "in_project"". Linking it to the heading of PRJ-A succeeded.
Unlinking in_project removed the under_heading edge in the same write.
`TestScoped*` covers clear, reject, reparent, delete, ACL on clears,
CreateLinked, PatchWithRelations, cyclic data and the no-automation count on
memstore, fsstore and sqlite locally; postgres runs in CI. Frontend:
scopedRelations unit tests; forms suite passes (1120 tests).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
