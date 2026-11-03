---
id: DOCS-PPFC30E
type: docs-checklist
title: 'Docs: TKT-V05706X'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why `Session.Cause` is never backfilled, why the inbox-page fields are hidden from MCP and CLI JSON, why `WaitingMessages` renders nothing under a reloaded config, the drawer start gate in `OpenChannel.vue`
- [x] Function/type docs if public API: godoc on `channels.Store.GetMessages`, `CheckGetMessages`, `GroupRefs`, `realtime.WaitingMessages`, the new `v1` wire types; comment-lint gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change; `rela channel inbox` output is unchanged)

## User-facing Documentation

- [x] `GUIDE-realtime.md` (generated to `docs/realtime.md` with `just docs`): the session cause in the storage paragraph (no backfill), "The waiting message" in the Inbox section, and that MCP and CLI do not carry the new fields
- [x] `GUIDE-data-entry.md` (generated to `docs/data-entry.md`): "Answering from the inbox" subsection
- [x] `docs/data-entry/api-reference.md`: the live inbox path carries the waiting fields; the summary does not

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] API docs updated (if applicable): OpenAPI `ChannelSession.cause` and the inbox entry's `actions`, `last_seq`, `message`, session `cause` and `transitions`; wire goldens regenerated
