---
id: IMPL-N7UTWCT
type: implementation-checklist
title: 'Implementation: Name the page and the app in the browser tab title'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (useDocumentTitle.test.ts, useTabTitleBadge.test.ts, gate tests in EntityDetail.header.test.ts and DynamicForm.embedded.test.ts)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: frontend only; the component tests mount EntityDetail and DynamicForm with the shell's title writer)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty page title skipped, page equal to app name, entity still loading, embedded form and entity panels leave the tab)
- [x] Error handling in place (errors surfaced, not swallowed) (usePageTitle throws outside a scope instead of leaking a title)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing mount helpers of each suite)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by component tests; no e2e asserts titles)
- [x] Each acceptance criterion verified with test scenario from planning (page · app, page part per screen, app alone without a title, badge in front, documented)
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (module-level outlet like usePageHeader)
- [x] Checked for DRY opportunities — repeated literals, expressions, or (PageTitle and the page header publish for most screens; views without either call usePageTitle once)
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (the tab shows only titles the page already shows to the same principal)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
