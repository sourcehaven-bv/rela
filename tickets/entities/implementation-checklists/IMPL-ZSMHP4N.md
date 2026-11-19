---
id: IMPL-ZSMHP4N
type: implementation-checklist
title: 'Implementation: A finished run loses its timeline and manage view'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestLogBody, the flowengine fakes, flowView and FlowRunCard tests)
- [x] Integration tests written (test full flow, not just units) (TestFlowControl_FinishedRunKeepsTimeline and TestFlowControl_FinishedRunWithoutLog run FlowControl on a real project)
- [x] Happy path implemented
- [x] Edge cases from planning handled (header without a body; unreadable body; removed flow version; forked run; reader who does not manage)
- [x] Error handling in place (errors surfaced, not swallowed) (endBody errors fail the end of the run; an unreadable log is logged and shown without a timeline)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: uses the existing flow test project helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] Feature manually tested end-to-end (through FlowControl.Run and SubjectRuns on a test project, and the mounted run card)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** TestFlowControl_FinishedRunKeepsTimeline: a completed
and a cancelled run keep their log; a manager gets the timeline, the diagram
with the final state marked and the manage action; a reader gets the timeline
without actions; the run list has no timeline. The cancel step names alice.
TestFlowControl_FinishedRunWithoutLog: no body and a bad body show the run
without a timeline. FlowRunCard shows a manager the timeline and diagram of a
finished run with no controls.
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (header writes go through PatchEntity; detail reads reuse the FlowCache)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (the log reaches the same readers as an active run's log; the manage action uses the same authorize check)
- [x] No silent failures (errors logged AND returned) (an unreadable log is logged; write errors are returned)
- [x] No debug code left behind
