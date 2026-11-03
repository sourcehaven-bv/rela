---
id: DOCS-4DBCHN
type: docs-checklist
title: 'Docs: Sessions 4/4: non-interrupting timers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the slot grid after the last post (`internal/realtime/reminders.go`), the shared cap and backlog bound (`timers.go`), pausing a reminder with broken props, and record pruning
- [x] Function/type docs if public API: godoc on `channels.ReminderRecord`, `ReminderPost`, `CheckReminders`, `SessionStep.Remind`, `realtimecfg.SessionReminder`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `rela channel` marks reminder messages and shows reminder due times

## User-facing Documentation

- [x] `GUIDE-realtime.md`: `session.reminders` (in, every, first, max, label, post), limits and refusals, the slot rhythm and missed slots, close/reopen/restore, reload and paused reminders, guards via `session.reminders['name']`, message protection, `reminders` on the session block; regenerated into `docs/realtime.md`
- [x] `GUIDE-mcp-server.md`: `reminders` on `read_channel`; regenerated into `docs/mcp-server.md`

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `reminders` on the session block and the `reminder_message` error
