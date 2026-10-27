---
id: DOCS-GUDO6P
type: docs-checklist
title: 'Docs: Inbox 2 read markers and unread counts'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: own-post marker moves and the file-store best-effort case, the capped count query, the activity-ordered window, per-reader live scoping
- [x] Function/type docs if public API: godoc on `channels.Store` SetRead, Reads, ListReads, `ReadState`, `realtime.Service.MarkRead`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel mark-read`, unread in `rela channel list` and `rela channel inbox`

## User-facing Documentation

- [x] `GUIDE-realtime.md`: read markers, unread counts, what counts, retention and id reuse; `GUIDE-mcp-server.md`: `mark_read` and inbox kinds; postgres guide upgrade note for migration 35; regenerated
- [x] `GUIDE-acl-security.md`: marker identity, read-only clients, id reuse; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `PUT .../_read`, `read` on channel list and head, inbox `unread_channels`, `unread_count`, `kinds`
