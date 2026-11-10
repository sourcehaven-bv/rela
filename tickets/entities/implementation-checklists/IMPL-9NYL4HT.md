---
id: IMPL-9NYL4HT
type: implementation-checklist
title: 'Implementation: Flows M1d: run flows end to end'
started: "2026-11-08"
completed: "2026-11-08"
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

- fs project in `.ignored/flowproj`: `rela flow start onboarding PER-001` started a run and wrote the outputs; a second start was refused as busy; `rela flow runs` (table and JSON) listed it; `rela flow cancel` cancelled it and the header recorded `cancelled`. The audit records of the start carry `by` with the OS user.
- `--run-as` on `flow start` is refused (identity edge).
- sqlite: `rela db import-fs` copied the unfinished run, which was then cancellable in the new database.
- Automated: flowengine tests cover failed outputs, failed creates, failed finish, backoff growth and orphan headers; `TestFlowControl_GatesStartCancelAndRuns` passes on fs, sqlite and postgres; fsimport tests cover copying runs and refusing without a run store.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
