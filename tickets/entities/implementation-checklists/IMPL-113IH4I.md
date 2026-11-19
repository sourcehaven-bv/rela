---
id: IMPL-113IH4I
type: implementation-checklist
title: 'Implementation: Link text and truncation for link properties'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (display_test.go (link_text needs link, blank link_text), property_display_test.go (wire field), UrlWidget.test.ts (own type, types agree, types disagree, title is href))
- [x] Integration tests written (test full flow, not just units) (e2e url-link.spec.ts 'shows link_text instead of the address' against the built server)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a value that is not a link shows as itself; types that disagree on link_text show the URL; long URLs truncate at 40ch in tables and nested values)
- [x] Error handling in place (errors surfaced, not swallowed) (a blank link_text and link_text without link are schema errors)

## Test Quality

- [x] Using fixture builders or factories for test data (the e2e fixture project gains one site_url property)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: none)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: none)

## Manual Verification

- [x] Feature manually tested end-to-end (the e2e test runs the real server and browser; the truncation width was measured in Chromium for a table, nested values and inline edit)
- [x] Each acceptance criterion verified with test scenario from planning (link text shown with the URL as target and title (unit and e2e); truncation (measured); schema errors (Go); docs (metamodel guide))
- [x] ~~Edge cases manually verified~~ (N/A: covered by the tests and the Chromium measurement)

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code) (follows the existing display.link and currency options end to end)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (link_text is schema text rendered by text interpolation; the href keeps the existing scheme check)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
