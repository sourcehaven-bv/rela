---
id: DOCS-RG6EF0
type: docs-checklist
title: 'Docs: Sessions 4/3: interrupting timers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: due-time computation and restart anchors (`internal/realtime/timers.go`), the DueWake and round scheduling (`duewake.go`), the filechannels untombstone ordering, and the stored `NextDueAt` as an index hint
- [x] Function/type docs if public API: godoc on `Store.SetSession`, `Store.ListSessionsDue`, `SessionStep.Due`, `realtime.Timers`, `DueWake`, `Resumption`, `channels.DamageWarner`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel` shows the due time

## User-facing Documentation

- [x] `GUIDE-realtime.md`: `after:` timers with `since: state|session`, limits, refused timer-only cycles, closed channels and reopen, restore, reload, degraded config, notices and audit `timer:<type>/<state>/<name>`, `due_at` and `timers` on the session block; regenerated into `docs/realtime.md`
- [x] `GUIDE-mcp-server.md`: `due_at` and `timers` on `read_channel`; regenerated into `docs/mcp-server.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `due_at` and `ChannelSessionTimer`
