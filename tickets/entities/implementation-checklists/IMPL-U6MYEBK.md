---
id: IMPL-U6MYEBK
type: implementation-checklist
title: 'Implementation: Declarative metric capture: schedule task that appends a dashboard card value as a record'
started: "2026-11-19"
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
Built rela, rela-server and the SPA from the worktree and ran a scratch fs project
(.ignored/e2e-capture): three tickets with points 1.5, 2.5 and 3.5, an avg card
"Mean points", a measurement record type (decimal value, scale 2) on metric
MET-points, and a capture task with run_as system:metrics.

- `rela scheduler` refused to start, naming the capture task.
- rela-server logged "scheduled capture", ran the task at once and logged
  "metric captured ... value=2.50". records/ held one row: value "2.50",
  recorded_by system:metrics, ts equal to the read time. GET
  /api/v1/_dashboard/card served the same "2.50".
- With the tickets misfiled (the store saw none), the run failed with "no row has
  a value for points", stored nothing and entered the retry ladder.

Unit and integration tests: internal/scheduler capture_test.go (config shapes,
schema checks, rounding, run_as attribution, failure stores nothing, kind
registered only with an evaluator); internal/dataentry dashboard_capture_test.go
(every display against a real App, refusals, row gate from the bound Request,
hidden-value refusal); internal/cli TestRefuseCaptureTasks. Lint, arch-lint,
comment-lint, plimsoll and archguard pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
