---
id: IMPL-VANTS
type: implementation-checklist
title: 'Implementation: Non-cards reverse relations silently ignored on data-entry forms'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new units — existing
RelationPicker.vue and DynamicForm.vue logic was already in place; the gap was
e2e coverage)
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

- AC1 (list incoming sources): existing test
`non-cards picker lists linked source entities with direction: incoming` passes
— TASK-001 is rendered as a tile in the "Implemented by" picker on FEAT-001.
- AC2 (add): new test
`non-cards picker add persists as peer --rel--> current entity` passes — picking
TASK-002 in the picker creates `TASK-002 --implements--> FEAT-001` (verified via
API listRelations on TASK-002 outgoing).
- AC3 (remove): new test
`non-cards picker remove deletes the underlying edge` passes — removing TASK-001
tile deletes the underlying edge (verified via API listRelations on TASK-001
outgoing post-save).
- AC4 (target-types/cardinality): exercised implicitly by AC1/AC2; the
picker pulls candidates from `relationType.from` (tasks) and accepts
multi-cardinality (TASK-001 and TASK-002 can both be linked).
- AC5 (e2e spec exists and passes): `e2e/tests/reverse-relations.spec.ts`
now has 6 tests, all passing. Full e2e suite: 177 passed, 1 skipped (unrelated).

Quality gates run from repo root:
- `frontend/ npm run typecheck` — clean
- `frontend/ npm run lint` — 0 errors (71 pre-existing warnings, none in
files touched by this ticket)
- `frontend/ npm run test:run` — 574 passed
- `e2e/ npm run lint` — clean
- `e2e/ npx playwright test` — 177 passed, 1 skipped

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
