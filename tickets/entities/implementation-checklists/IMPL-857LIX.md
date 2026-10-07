---
id: IMPL-857LIX
type: implementation-checklist
title: 'Implementation: Create form cannot save a pre-linked peer whose type uses a dashed id_prefix'
started: "2026-10-06"
completed: "2026-10-06"
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
- `e2e/tests/create-prelink.spec.ts` fails on develop with the reported
toast ("Save aborted: could not resolve the entity type ... for \"contains\"").
With only the type fix it saved the edge backwards (`source_type_not_allowed`,
`target_type_not_allowed` warnings, section empty). With the full fix it passes
and the module's outgoing `contains` lists the new task.
- New `DynamicForm.embedded.test.ts` cases fail on develop (3 of 3) and pass
with the fix.
- Frontend vitest (forms, entity, utils): 1898 passed. E2E full suite run:
420 passed; 4 failures were caused by a concurrent server rebuild and pass on
rerun.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
