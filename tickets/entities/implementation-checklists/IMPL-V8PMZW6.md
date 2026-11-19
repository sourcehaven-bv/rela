---
id: IMPL-V8PMZW6
type: implementation-checklist
title: 'Implementation: Flow run list shows a run whose job failed as Working'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestFailedJobs_AgreesWithExplain)
- [x] Integration tests written (test full flow, not just units) (TestFlowJob_RunListShowsFailedJob runs a webhook job that fails on a test server and compares the list with the run detail)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a run in its initial state with no stored instance; a forked run with a failed branch; a stranded run or unreadable version is left as its header says, as on the run page)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: the one new read returns its error to the caller)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: uses the existing job project and job harness)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: the list status is compared with the detail status)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** TestFlowJob_RunListShowsFailedJob builds a real
project, fails the webhook job with a 400 and reads the run list and the run
detail through FlowControl, the service the data-entry routes call: both say
failed and the header still says working. The flowengine and appbuild suites
pass.

## Quality

- [x] Code follows project patterns (check similar code) (one run store read per page, as forkedRuns does for the task list)
- [x] Checked for DRY opportunities (place() and FailedJobs share jobFailed)
- [x] No security issues introduced (the runs read are those whose headers passed the row gate)
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: no new failure path)
- [x] No debug code left behind
