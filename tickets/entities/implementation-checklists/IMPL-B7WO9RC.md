---
id: IMPL-B7WO9RC
type: implementation-checklist
title: 'Implementation: Flows M5d: entity-change triggers in flows.yaml and delete internal/automation'
started: "2026-11-14"
completed: "2026-11-15"
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

- `rela migrate config` moved the automations of tickets/, examples/basecamp,
internal/apptest/testdata/project, internal/dataentry/testdata/golden/fixture
and internal/configedit/testdata into flows.yaml `triggers:`; `rela validate`
passes on all five.
- A schema.yaml that still declares `automations:` (directly or through an
include) is refused at load and by `rela validate` with the
`automations-to-flows` migration message.
- An invalid trigger fails the server boot with an error that starts with
"flows.yaml: triggers:".
- The ticket automations (checklist creation on status change) fire from
tickets/flows.yaml: the golden synthetic-flows jsonl outputs are unchanged.
- `go test ./...`: every package passes except cmd/rela-desktop
TestChromeStyle_TargetsShippedClasses, which needs the built frontend
(internal/dataentry/static holds only favicon.svg in this worktree).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
