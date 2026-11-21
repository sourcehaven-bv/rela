---
id: IMPL-8KA9VYP
type: implementation-checklist
title: 'Implementation: Manual start condition, mail button and hiding empty dashboard cards'
started: "2026-11-21"
completed: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestParse_Problems manual cases, TestRender_Button, TestMailRender_ButtonErrors, TestValidateDashboardCardHideEmpty, DashboardView hide_empty tests)
- [x] Integration tests written (test full flow, not just units) (TestFlowControl_ManualStartWhen on a project on disk through FlowControl; TestMailRender_MatchesDirectRenderer pins the Lua binding with a button to the Go renderer; compat test scores the button)
- [x] Happy path implemented (condition holds: flow offered and started; button rendered; non-empty card shown)
- [x] Edge cases from planning handled (caller without permission refused as forbidden first; relative URL without a base URL refused; failed hide_empty card still shown)
- [x] Error handling in place (errors surfaced, not swallowed) (start refused with a conflict naming the flow and subject; mail.render raises on a bad button; config load refuses hide_empty on a line card)

## Test Quality

- [x] Using fixture builders or factories for test data (stepProjectFlows, addFlowPerson, card() and seedDashboard helpers)
- [x] No hardcoded values in assertions when object is in scope (assertions compare against the configured titles and URLs)
- [x] Only specifying values that matter for the test (only status, label, URL and hide_empty vary)
- [x] Interpolated values constructed from objects, not hardcoded (expected texts follow from the input given in the same test)
- [x] Property comparisons use original object, not hardcoded strings (card titles come from the seeded cards)

## Manual Verification

- [x] Feature manually tested end-to-end (see evidence below)
- [x] Each acceptance criterion verified with test scenario from planning (manual start, mail button and hide_empty each run by hand)
- [x] Edge cases manually verified (the empty card reappears once its list has a row)

**Verification Evidence:**
<!-- Document what you tested and the results -->

- Flow: test project with `when: "entity.status ~= 'left'"` on the manual
start. `rela flow start onboarding PER-001` (status left) answered
"conflict: flows: the start condition does not hold"; PER-002 (status new)
started RUN-BT8A.
- Mail: `rela script` with mail.render and a button printed the text part
"Open step: https://example.test/s/portal" and an `<a class="btn">` with
white text inside a `td.btn-cell` with the accent background.
- Dashboard: rela-server with a `hide_empty` card on an empty list showed
only "People" and "Leavers shown"; after one person matched, the
"Leavers" card appeared with 1.

## Quality

- [x] Code follows project patterns (check similar code) (ManualStarts mirrors OnCreate.anyHolds; Button reuses safeHref; HideEmpty sits beside Width and Permission)
- [x] Checked for DRY opportunities (openPermissions is shared by Start and Startable) — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (permission check stays first; button URL allowlisted and label escaped; no new raw HTML path)
- [x] No silent failures (errors logged AND returned) (a guard evaluation error is logged and the start is not offered, as for create starts)
- [x] No debug code left behind (none)
