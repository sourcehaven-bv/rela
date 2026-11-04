---
id: IMPL-C416UJ3
type: implementation-checklist
title: 'Implementation: Reject type-disallowed relation edges on every write surface'
started: "2026-11-04"
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
Worktree build of rela-server on the #1992 schema (answer, ask, task;
`answers` from answer to ask, max_outgoing 1):

- PATCH answer with `answers` to a task: 422 target_type_not_allowed, title
  unchanged, no edge.
- PATCH task with inverse `answered_by` to an answer: 422
  target_type_not_allowed.
- POST answer with `answers` to a task: 422; GET new-answer returns 404, so
  no entity was created.
- Per-edge POST `/answers/rest-answer/relations/answers`: 422.
- PATCH answer with `answers` to an ask: 200, edge written.
- `rela link cli-answer answers cli-task`: refused with "target entity type
  not allowed", no relation file written.

Automated: TestPatchRelations_RejectsDisallowedEndpointTypes (14 cases:
canonical and inverse, bounded and unbounded, wrong source and target),
TestCreateEntity_RejectsDisallowedEndpointTypes,
TestCreateRelation_RejectsDisallowedEndpointTypes,
TestCreateRelation_RefusesTypeMismatch, TestReplaceRelations_RefusesTypeMismatch.
Each asserts no mutation and no audit record.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
