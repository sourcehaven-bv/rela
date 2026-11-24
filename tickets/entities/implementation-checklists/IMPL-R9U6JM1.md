---
id: IMPL-R9U6JM1
type: implementation-checklist
title: 'Implementation: Body task counts as a roll-up (rollup body_tasks)'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: small inline seeds, as in the sibling roll-up tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Scratch fs project with a `todo` type declaring
`tasks_done` and `tasks_total`; `rela create todo -B` with a body of two
top-level tasks, one nested `* [X]` task and a fenced task; `rela list todo -o
json` served `tasks_done: 2`, `tasks_total: 3`. Automated: TestCountTasks,
TestCompile_BodyTasks, TestParse_BodyTasksShape, TestRollups_BodyTasks,
TestRollups_BodyTasksCostIsScaleFree, TestBodyTasks_ListColumn.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
