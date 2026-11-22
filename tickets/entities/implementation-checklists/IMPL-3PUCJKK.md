---
id: IMPL-3PUCJKK
type: implementation-checklist
title: 'Implementation: Flow run lists show subject ids and unstyled links'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestWholeDisplayTitle, FlowLists.test.ts)
- [x] Integration tests written (test full flow, not just units) (TestFlowControl_SubjectTitle and TestFlowControl_SubjectTitleHidden run FlowControl on a real project with acl.yaml, through RunList, Tasks, Run and SubjectRuns)
- [x] Happy path implemented
- [x] Edge cases from planning handled (finished run, unreadable subject, a hidden display property, one hidden placeholder of a template, an unparsable address, a flow the list does not know)
- [x] Error handling in place (errors surfaced, not swallowed) (the header batch error is returned)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: uses the package's stepProject and addFlowPerson helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: the expected titles are the contract under test)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no object comparison)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the server tests run the real FlowControl with the ACL; the view tests mount both tables)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: covered by the tests above)
- [x] ~~Edge cases manually verified~~ (N/A: covered by the tests above)

**Verification Evidence:** TestFlowControl_SubjectTitleHidden fails with plain
DisplayTitle (actual "Ada" for a template with a hidden placeholder) and passes
with WholeDisplayTitle.

## Quality

- [x] Code follows project patterns (check similar code) (one header batch per page, as collection-reads.md asks)
- [x] Checked for DRY opportunities (the whole-title rule moved from two dataentry helpers into metamodel.WholeDisplayTitle)
- [x] No security issues introduced (titles come from the gated header read; a hidden title property gives the address)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
