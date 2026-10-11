---
id: IMPL-18CA36
type: implementation-checklist
title: 'Implementation: Lua HTML and markdown conversion helpers'
started: "2026-10-09"
completed: "2026-10-09"
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

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

## Notes

- Scope: `rela.md.from_html` and `rela.md.to_html`, backed by the leaf package `internal/htmlmd`. Out of scope: images and attachments (dropped and reported as lossy).
- Approach: bluemonday allowlist shared by both directions; html-to-markdown v2 (commonmark, strikethrough, table) for HTML to markdown; goldmark (no raw HTML, strikethrough, table) for markdown to HTML. `FromHTML` returns a fixed point, so a round trip never changes the text again. Both return a lossy flag computed from the allowlist.
- Alternatives: a hand-written converter (rejected: more code, worse coverage); passing raw HTML through (rejected: unsafe).
- Security: remote HTML is untrusted and sanitized before conversion; markdown output is sanitized after rendering; links only http, https and mailto; 1 MiB input cap.
- Tests: `internal/htmlmd` table tests for both directions, loss flags, entity escaping, input limit and `TestRoundTripStable` (reviewer's breaking inputs included); `TestMdHTMLConversion` for the bindings.
- Review: cranky-code-reviewer found three significant issues (intraword emphasis, entity-like text, silent loss) and four smaller ones; all addressed, see the review responses.
- Docs: Lua scripting guide, "Rich text" under Sync connectors and the rela.md reference.
