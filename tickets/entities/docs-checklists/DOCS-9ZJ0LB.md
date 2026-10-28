---
id: DOCS-9ZJ0LB
type: docs-checklist
title: TKT-9YRRM5 docs
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the inbox grid and clock, overflow deferral, staggered safety slices, the per-round memo, durable versus ephemeral marks
- [x] Function/type docs if public API: godoc on `SetInboxGrid`, `ChannelsScan`, `SubscribeLive`, the summary view; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-realtime.md`: summary route, live updates, grid sizes, safety replay, one-process limit; `GUIDE-data-entry.md`: Inbox section; API reference: inbox live paths; regenerated
- [x] `GUIDE-acl-security.md`: per-principal memo, summary, frame timing, overflow deferral; regenerated

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `GET /api/v1/_inbox/summary`
