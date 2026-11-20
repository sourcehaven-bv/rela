---
id: IMPL-C66G7EP
type: implementation-checklist
title: 'Implementation: The Changes list shows raw actors and relation names; History opens empty'
started: "2026-11-20"
completed: "2026-11-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (audit_titles_test.go: TestChangeTitles_NamesActorsByTitle, TestActorMayBeEntity; changeText.test.ts actor and incoming cases; useHasHistory.test.ts; HistoryView.origin.test.ts subject cases)
- [x] Integration tests written (test full flow, not just units) (ChangesPanel.test.ts renders a person's title from the response's titles map; EntityDetail.world.test.ts checks History in both blocks for an empty timeline; audit_budget_test.go still pins the page's store reads with users in the batch)
- [x] Happy path implemented
- [x] Edge cases from planning handled (the entity on screen as its own actor; unreadable person ids stay as sent; reserved, integration, webhook and unknown users are never looked up; an inverse without a label reads as from its source; an answer for an address no longer on screen is dropped; a gone live entity falls back to type and id)
- [x] Error handling in place (errors surfaced, not swallowed) (a failed title lookup is logged and serves no titles, as before; a failed timeline read hides the button and the history page keeps its own error display)

## Test Quality

- [x] Using fixture builders or factories for test data (seedEntity, mustNewACL, the existing mountDetail and mountWith helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: the expected texts are the i18n output under test)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions compare rendered text and title maps)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the database build with an audit log and a principal_property project is not run locally; the route, the panel, the button and the page are covered by handler and component tests)
- [x] Each acceptance criterion verified with test scenario from planning (actor titles: TestChangeTitles_NamesActorsByTitle and ChangesPanel; identity labels: changeText cases; incoming relations: changeText cases; History hidden: EntityDetail.world and useHasHistory; title on the history page: HistoryView subject tests; docs updated in audit-log, data-entry and api-reference)
- [x] ~~Edge cases manually verified~~ (N/A: covered by the tests above)

**Verification Evidence:**
Go dataentry package tests pass; frontend 444 files / 6544 tests pass; typecheck, ESLint, golangci-lint, comment-lint, arch-lint and markdownlint are clean.

## Quality

- [x] Code follows project patterns (check similar code) (users join the existing gated title batch; inverse labels follow RelationCards; i18n keys are literal)
- [x] Checked for DRY opportunities — repeated literals, expressions, or (the webhook prefix is one constant used by the receiver and the title batch)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (actor titles come from the redacting resolver in the request's world and only whole; an unreadable person is absent, as for relation ends; integration and webhook names are configuration)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
