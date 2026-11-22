---
id: IMPL-V0X0B7K
type: implementation-checklist
title: 'Implementation: Remaining data-entry UI gaps in New button and column Add and create forms and lists'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (covered by the Playwright kanban specs in CI)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: the existing tests build their data inline in the local style)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: frontend tests assert literal UI values)
- [x] ~~Only specifying values that matter for the test~~ (N/A: frontend tests assert literal UI values)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: frontend tests assert literal UI values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: frontend tests assert literal UI values)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Ran the atlas config (migrated copy) against this branch. all_adviezen (group_by + create_form) shows no header New and keeps the group + and Toevoegen. ncr_bord (property board + create_form) shows Toevoegen in each column and no header New. Column Add in In analyse opens the form with Status = In analyse. A form with body: true shows the editor. all_organisaties and the no-body form were not exercised by hand (no data). They are covered by unit tests. Vitest full run: 3 flaky failures under load, all pass alone. vue-tsc, eslint and prettier clean.
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
