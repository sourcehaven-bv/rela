---
id: DOCS-B40HN5
type: docs-checklist
title: 'Docs: TKT-O6RC8Z'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the flag is stored at post time, why signal types never count, how the link reveal waits for the channel and clears itself
- [x] Function/type docs if public API: godoc on `NoUnread`, `MessageType`, `CountsUnread`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: `rela validate` gains a warning, no flag change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` documents `unread: false` (true or false only, new posts only, refused on signals, brief mixed flags during a rolling reload) with an example pinned by `TestDocsExample_Unread`; `GUIDE-data-entry.md` notes that `display: hidden` does not stop counting and warns; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] OpenAPI description of `read.unread` updated in `internal/openapi/channels.go`
