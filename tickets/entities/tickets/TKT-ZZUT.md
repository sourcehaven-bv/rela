---
id: TKT-ZZUT
type: ticket
title: Markdown renderer preserves source line breaks in data-entry view
kind: enhancement
priority: medium
effort: xs
status: done
---

## Description

The data-entry SPA renders entity content markdown with `breaks: true` passed to
marked.js (`frontend/src/utils/markdown.ts:50`). That option converts every soft
line break (single newline) in the source markdown into a `<br>` tag. Because
authors commonly hard-wrap markdown source at ~80 chars for git-friendly diffs,
the rendered HTML inherits the source's column width and the visible line breaks
bear no relation to the viewport width.

Expected (CommonMark default): soft line breaks become whitespace; paragraphs
are separated by blank lines; only two trailing spaces (or a backslash) produce
a `<br>`.

Fix: set `breaks: false` (or drop the option).

## Acceptance

A paragraph wrapped across multiple lines in the source renders as one
continuous paragraph that reflows with the viewport width. Two trailing spaces
still produce a `<br>` (CommonMark hard break). Code blocks, lists, headings
remain unaffected.
