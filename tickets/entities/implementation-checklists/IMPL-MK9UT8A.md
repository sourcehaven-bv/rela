---
id: IMPL-MK9UT8A
type: implementation-checklist
title: 'Implementation: Flow run page repeats the flow and shows person ids and wrong wording'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (flowView, FlowRunCard and sidebarNav tests)
- [x] Integration tests written (test full flow, not just units) (TestFlowControl_TimelineNamesActor runs FlowControl on a real project with three ACL policies)
- [x] Happy path implemented
- [x] Edge cases from planning handled (hidden title; no person mapping; a count of one; a job still running keeps the no-roles line)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: no new error path; the title batch error is returned as before)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: uses the existing flow test project helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] Feature manually tested end-to-end (through FlowControl.Run on a test project and the mounted run card; the portal run was reported in the harbourmaster test)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** TestFlowControl_TimelineNamesActor: a mapped person
is named for a reader who may see the display field and not for one who may not,
and a login is not looked up without person mapping. The FlowRunCard test sees
'after 1 failed attempt:' and 'after 3 failed attempts:', no no-roles line on a
failed job, and no title with hideTitle. The sidebarNav test lights a space home
row only on the home page.
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (reuses the gated title batch and pilePersonType)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (subjects and actors share one title batch)
- [x] No security issues introduced (titles come from the gated header read; a hidden display field falls back to the id and is dropped)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no new failure path)
- [x] No debug code left behind
