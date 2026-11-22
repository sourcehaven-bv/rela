---
id: IMPL-X3DUFDV
type: implementation-checklist
title: 'Implementation: Flows panel offers Start without the start permission'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestFlowsAPI_SubjectRunsStartable, FlowRunsPanel.test.ts, flows.test.ts)
- [x] Integration tests written (test full flow, not just units) (TestFlowControl_Startable runs FlowControl on a real project with acl.yaml for a holder, a reader without the permission and a user who cannot read the subject)
- [x] Happy path implemented
- [x] Edge cases from planning handled (an unreadable subject is not found; the list does not apply per_subject, which the panel still does; a failed load hides Start)
- [x] Error handling in place (errors surfaced, not swallowed) (any error other than a missing permission is returned)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: uses the package's stepProject and addFlowPerson helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: the flow names are the contract under test)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no object comparison)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the server test runs the real FlowControl with the ACL and checks Startable agrees with Start; the panel test mounts the panel)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: covered by the tests above)
- [x] ~~Edge cases manually verified~~ (N/A: covered by the tests above)

**Verification Evidence:** The two new panel tests fail on the panel of PR #2451
and pass with the fix. TestFlowControl_Startable asserts that Start is refused
for each user Startable leaves out.

## Quality

- [x] Code follows project patterns (check similar code) (Startable uses the same authorize call as Start)
- [x] Checked for DRY opportunities (Startable reuses manualPermissions and authorize)
- [x] No security issues introduced (the list is a UX hint; Start still checks the permission on the server)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
