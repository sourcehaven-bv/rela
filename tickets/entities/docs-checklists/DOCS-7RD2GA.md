---
id: DOCS-7RD2GA
type: docs-checklist
title: 'Docs: Comments across markdown block boundaries'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (phase 3 floors and long-endpoint check in `crossblock.go`; segment edge rules in `quotefind/segments.go`; overlap and code-scan rules in `commentHighlight.ts`)
- [x] Function/type docs if public API (`textanchor.Document`, `quotefind.Segments`, `comments.Body`, `comments.Span`, `anchorWire.Segments`)

## Project Documentation

- [x] README updated (if applicable) (textanchor README covers the prepared document, phase 3 and `quotefind.Segments`)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new project-wide pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from commits)
- [x] API docs updated (if applicable) (`docs/comments.md`: cross-block selections, what is not highlighted, nested comments, the `segments` wire field, the paragraph limit)
