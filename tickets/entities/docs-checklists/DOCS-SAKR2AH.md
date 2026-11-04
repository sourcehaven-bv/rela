---
id: DOCS-SAKR2AH
type: docs-checklist
title: 'Docs: TKT-LKIVLR2'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the quiet-notice rule and the `pending` join in `channelFold.ts`, why notices are always announced and when focus is handed on in `RlChannel.vue`, why a working state with a quiet cause puts decisions in the header in `useSessionDecisions.ts`
- [x] Function/type docs if public API: `ChannelNoticeView.toStatus`, `ChannelFoldRow.count` and `moves`, the `moves` prop of `RlChannelFold`, the `channelFold` message signature

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`, "Quiet message types": the run rule for session notices, the cases that stay in view, decision placement in a working state, the announcement exception; regenerated with `just docs`
- [x] ~~Security guide~~ (N/A: client-side display only; no ACL or server change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
