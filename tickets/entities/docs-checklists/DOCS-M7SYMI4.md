---
id: DOCS-M7SYMI4
type: docs-checklist
title: 'Docs: TKT-56TPOTE'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why a re-entry keeps `Session.Previous` and why it is never backfilled (`channels/session.go`), the compact CSS rules and why the short list is removed when the full list unfolds (`RlChannelSession.vue`), why the SPA drops an undeclared `previous` (`channelView.ts`)
- [x] Function/type docs if public API: godoc on `SessionPresentation`, `SessionDisplay`, `Session.Previous`, `SessionView.Previous`, `v1.ChannelSession.Previous`; TSDoc on the `compact` prop, `ChannelSessionView.previous` and `channelSessionContextItem`; comment-lint gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change; JSON output gains `previous` through the shared session view)

## User-facing Documentation

- [x] `GUIDE-data-entry.md` (generated to `docs/data-entry.md` with `just docs`): new "Compact session header" section with the example, the values, what the reader sees, the narrow behaviour, screen reader text and the load checks
- [x] `GUIDE-realtime.md` (generated to `docs/realtime.md`): the stored `previous` in the sessions storage paragraph, the corrected "rela keeps only the current state" sentence, a pointer from the narrow-panel paragraph to the new key, and `cause` and `previous` in the session JSON example

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `ChannelSession.previous`; wire goldens regenerated
