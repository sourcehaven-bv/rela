---
id: DOCS-X29X5B
type: docs-checklist
title: 'Docs: TKT-TU3XH4'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why a busy column cannot be first or sortable, why the export skips it, the refused-move announcement
- [x] Function/type docs if public API: `ListColumn` godoc names all four column kinds; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` Column Options gains the busy column and an example (pinned by `TestDocsExample_BusyListColumn`); `GUIDE-realtime.md` Busy indicator mentions lists; regenerated with `just docs`; `docs/data-entry/api-reference.md` one-sentence edit
- [x] ~~Security guide~~ (N/A: no ACL change; same gates as the kanban busy field)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; `?busy=` already documented, list use noted in api-reference.md)
