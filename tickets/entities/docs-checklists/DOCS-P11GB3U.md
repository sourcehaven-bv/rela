---
id: DOCS-P11GB3U
type: docs-checklist
title: 'Docs: TKT-48S88RB'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why only the notifier key takes hidden delivery (TASK-550RD), why the scope is base plus user, why reports stop after pagehide, why the desktop shell reports unsupported
- [x] Function/type docs if public API: doc comments on `useInboxNotifications`, `readInboxIntoSummary`, `deliverLive`, `startTabFocus`, `diffInbox`, the prefs helpers

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention; the hidden-delivery opt-in is documented at `registerLive`)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: new "Tab title and browser notifications" section under Inbox, and a sentence on the Settings page; `GUIDE-realtime.md`: pointer from the Inbox section; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: no ACL change; notifications read only the gated inbox)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
