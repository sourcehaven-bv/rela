---
id: IMPL-J0HTTVX
type: implementation-checklist
title: 'Implementation: Finished flow run page hides its subject'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestFlowControl_FinishedRunNamesSubject)
- [x] Integration tests written (test full flow, not just units) (the test starts and cancels a real run through FlowControl on the on-disk project and reads it back through Run and SubjectRuns)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a reader who cannot read the subject still gets not found; the subject comes from the gated edge, so a hidden edge leaves it empty)
- [x] Error handling in place (errors surfaced, not swallowed) (an edge read error fails the call)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: reuses the stepProject and addFlowPerson helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: the subject id is the fixture's own literal)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** rela-server (fs build) on a copy of the harbourmaster
project with a staff person in GRP-STAFF and a customer linked by customer-of.
Staff started a signup run on ACC-001 and cancelled it. `GET
/api/v1/_flows/runs/RUN-B5PH` then returned `"subject":"ACC-001"` for both staff
and the customer; before the fix the field was missing. The harbourmaster
database itself held nine runs, eight of them finished, which matches the
report.

## Quality

- [x] Code follows project patterns (check similar code) (same `carried` helper as RunList)
- [x] Checked for DRY opportunities (reuses `carried`; the split into active and finished runs moved to splitRuns)
- [x] No security issues introduced (the subject comes from the gated `_carries` edge of a header that passed the run gate)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
