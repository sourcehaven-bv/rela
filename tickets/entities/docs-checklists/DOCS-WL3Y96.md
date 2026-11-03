---
id: DOCS-WL3Y96
type: docs-checklist
title: 'Docs: TKT-LAMZ8F'
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] Changelog entry added
- [x] API docs updated (if applicable)

## Notes

- Scope: `rela.md.from_html` and `rela.md.to_html`, backed by the leaf package `internal/htmlmd`. Out of scope: images and attachments (dropped and reported as lossy).
- Approach: bluemonday allowlist shared by both directions; html-to-markdown v2 (commonmark, strikethrough, table) for HTML to markdown; goldmark (no raw HTML, strikethrough, table) for markdown to HTML. `FromHTML` returns a fixed point, so a round trip never changes the text again. Both return a lossy flag computed from the allowlist.
- Alternatives: a hand-written converter (rejected: more code, worse coverage); passing raw HTML through (rejected: unsafe).
- Security: remote HTML is untrusted and sanitized before conversion; markdown output is sanitized after rendering; links only http, https and mailto; 1 MiB input cap.
- Tests: `internal/htmlmd` table tests for both directions, loss flags, entity escaping, input limit and `TestRoundTripStable` (reviewer's breaking inputs included); `TestMdHTMLConversion` for the bindings.
- Review: cranky-code-reviewer found three significant issues (intraword emphasis, entity-like text, silent loss) and four smaller ones; all addressed, see the review responses.
- Docs: Lua scripting guide, "Rich text" under Sync connectors and the rela.md reference.
