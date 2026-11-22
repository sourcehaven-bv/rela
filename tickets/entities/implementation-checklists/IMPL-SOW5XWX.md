---
id: IMPL-SOW5XWX
type: implementation-checklist
title: 'Implementation: Long property values on phones; HTML answers taken as API data'
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (htmlResponse.test.ts through the real axios instance)
- [x] Integration tests written (test full flow, not just units) (mobile-layout.spec.ts on the phone viewport, display and inline-edit sections)
- [x] Happy path implemented
- [x] Edge cases from planning handled (empty bodies, JSON, action text/CSV/XML bodies, blob and text response types, markup without a content type)
- [x] Error handling in place (errors surfaced, not swallowed) (an HTML answer rejects with an ApiError and a translated message)

## Test Quality

- [x] Using fixture builders or factories for test data (the e2e api fixture and SEED; one adapter helper in the unit test)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** The e2e test failed before the CSS change (the value
ran past the "Task" section) and with only the dd rule (it ran past "Narrow
fields"); it passes with both. Disabling the interceptor check fails the three
HTML unit tests. Full vitest, vue-tsc, eslint and the mobile-layout,
view-section-render-mode and entity-detail e2e specs pass.

## Quality

- [x] Code follows project patterns (check similar code) (the check sits beside the provisioning check in the response interceptor; the wrap rule in the shared properties-list.css)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (a stricter client check and CSS only)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
