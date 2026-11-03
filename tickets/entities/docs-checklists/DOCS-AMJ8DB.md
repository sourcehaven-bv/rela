---
id: DOCS-AMJ8DB
type: docs-checklist
title: 'Docs: Inbox 1 inbox read with waiting entries'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: gate-before-count order in `visibility.HolderLookup`, the ceiling and cursor in `internal/realtime/inbox.go`, wake pacing in the MCP watch
- [x] Function/type docs if public API: godoc on `channels.Store.ListWaiting`, `WaitingQuery`, `visibility.HolderLookup`, `realtime.CheckInboxLimit`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the gantt rule as written)
- [x] Help text accurate (if CLI changes): `rela channel inbox`

## User-facing Documentation

- [x] `GUIDE-realtime.md`: the inbox, entry shape, paging, limits, job identities; `GUIDE-mcp-server.md`: `list_inbox` and its use with `watch`; regenerated
- [x] `GUIDE-acl-security.md`: the `inbox_too_large` ceiling and what it discloses; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `GET /api/v1/_inbox`, `inbox_too_large`, `invalid_cursor`
