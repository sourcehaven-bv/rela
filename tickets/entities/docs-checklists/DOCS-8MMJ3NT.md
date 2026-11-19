---
id: DOCS-8MMJ3NT
type: docs-checklist
title: 'Docs: TKT-IO9PTLD'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: UrlWidget explains why it shows the URL when types disagree on link_text
- [x] Function/type docs if public API: godoc on `PropertyDisplay.LinkText` in metamodel and apiwire

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] The metamodel guide documents `display: {link_text: ...}`, its rules and the truncation; `docs/metamodel.md` regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: link_text is schema text; the href keeps its scheme check)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the schema endpoint gains a field documented in the Go wire type)
