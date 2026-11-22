---
id: IMPL-B0AEAJJ
type: implementation-checklist
title: 'Implementation: Link display for URL string properties'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** The Playwright spec `e2e/tests/url-link.spec.ts`
drives the real server and SPA. It checks an https value as a link (href,
target, rel) on the detail page, a list cell and a kanban card, clicks the card
link and sees a popup with the URL while the board stays put, and checks the
form input. A `javascript:` value shows as text on the same surfaces and the
form still saves it unchanged. The kanban e2e specs (39 tests) still pass with
the new fixture board. Go tests for metamodel, dataentry, dataentryconfig,
configedit and apiwire pass; vitest passes (126 files).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
