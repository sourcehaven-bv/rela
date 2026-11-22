---
id: IMPL-LBM9RPD
type: implementation-checklist
title: 'Implementation: The Changes list shows customers internal records and raw names'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (auditchanges_test.go (managed names, renumbers, hidden updates, paging and cursor), audit_titles_test.go, changeText.test.ts, ChangesPanel.test.ts)
- [x] Integration tests written (test full flow, not just units) (audit_handler_test.go runs the route and checks the titles map; audit_budget_test.go pins the store reads of a page)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a page is not shortened by left-out records, and a full page gets no cursor when only left-out records follow; a templated title with a hidden placeholder falls back to the id)
- [x] Error handling in place (errors surfaced, not swallowed) (a failed title lookup is logged and serves no titles; a disconnected client skips it)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing entityEntry, relationEntry and seedEntity helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions compare seqs and rendered text)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the route and the panel are covered by handler and component tests; the database build is not run locally)
- [x] Each acceptance criterion verified with test scenario from planning (each criterion has a test: managed and hidden-only records, labels and titles, the header_buttons gate; docs updated)
- [x] ~~Edge cases manually verified~~ (N/A: covered by the tests above)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (titles resolve through the gated resolver as mentions do; i18n keys are literal)
- [x] Checked for DRY opportunities — repeated literals, expressions, or (the panel computes each row once)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (titles come from the redacting reader in the request's world and only whole; the server leaves out more, never less)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
