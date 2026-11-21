---
id: IMPL-8TC5XH9
type: implementation-checklist
title: 'Implementation: Failed flow runs hide their error and blocked steps give no field feedback'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (title_test.go, TestCheck_Fields, TestJobErrorText, TestRedact, FieldShell and FlowRunCard tests)
- [x] Integration tests written (TestFlowControl_CompleteStepFieldFeedback, TestFlowJob_FailedJobForManagers, TestFlowJob_FailedTasksBudget, the write refusal handler test)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a template reading a restricted property, a subject withheld from the reader, missing template value, long titles, invalid UTF-8 in a job error, a rule that reads no field of the form, a forked run's job error)
- [x] Error handling in place (an unreadable secret store withholds the job error; errors returned, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: the flowcfg testdata project with small edits)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the saas-dev test environment is read-only for this change; the appbuild tests run the flows end to end on an on-disk project)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: covered by the Go and vitest tests listed above)

**Verification Evidence:**
The appbuild tests run the onboarding and hook flows on an on-disk project:
a manager reads the webhook error, a reader does not and gets the flow's
`failed_message:`; a step write with a lower-case value answers the pattern
warning at `name`; a rule refusal carries `fields: [stage]` and the rule's
description; the run stores "Onboarding" and its view is titled "Onboarding: ADA". The FlowRunCard tests mount
the card and check the inline feedback, focus, the blocked notice and the
retrying line.

## Quality

- [x] Code follows project patterns (flowview DTO leaf, writerule compile step, FieldShell feedback slots)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (job error only to managers and redacted; the title is held to visible: at load)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
