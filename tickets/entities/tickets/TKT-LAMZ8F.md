---
id: TKT-LAMZ8F
type: ticket
title: Lua HTML and markdown conversion helpers
kind: enhancement
priority: medium
effort: m
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

## Description

Add `rela.md.from_html(html)` and `rela.md.to_html(markdown)` so a sync
connector can keep markdown in rela while the remote system stores HTML
(Basecamp rich text).

- `from_html` converts with JohannesKaufmann/html-to-markdown.
- `to_html` renders with goldmark (raw HTML off) and sanitizes with a bluemonday allowlist.
- The pair must be round-trip stable: `from_html(to_html(md))` equals a normalized `md`, and `to_html(from_html(h))` is stable after one pass. Sync loop prevention compares against the base, so an unstable round trip would push forever.
