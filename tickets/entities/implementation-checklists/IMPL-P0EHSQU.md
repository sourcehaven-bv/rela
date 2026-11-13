---
id: IMPL-P0EHSQU
type: implementation-checklist
title: 'Implementation: Flows M5b: compile automations onto entity-change triggers'
started: "2026-11-13"
completed: "2026-11-13"
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

- Golden tests recorded against the old engine in commit 02937e71a.
`TestSchemaAutomations_Golden` (tickets, docs-project, basecamp, dataentry
fixture) passes unchanged. In `TestSyntheticAutomations_Golden`, 42 of 516 lines
change, and a script that strips the "automation condition could not be
evaluated" warnings from both sides finds them identical: the only difference is
the intended one.
- New tests: `internal/flow/fire_test.go` (order, false guard, failed guard
reported in place), `TestMayFireCoversSelects` (the event index is a superset of
`selects` over 128 trigger-key combinations and 8 events), and
`TestProcess_GuardRunsOnlyWhenSelected` (no bind and no warning on an unselected
write).
- Race tests pass on `internal/automation`, `internal/autocascade`,
`internal/flow`, `internal/entitymanager`, `internal/appbuild` and
`internal/dataentry`; the full `go test -race ./...` passed on the golden
commit.
- CLI smoke run with a binary built from the branch on a scratch copy of the
tickets schema: a ticket moved through ready, planning, in-progress and review
created the planning, implementation and review checklists with their `has-*`
relations; moving back to planning created no duplicate (`if_exists: skip`); a
bug moved to analyzing created its analysis checklist. The audit log attributes
each created entity and relation to `automation:<name>` in `triggered_by`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
