---
id: DOCS-0CDGCG7
type: docs-checklist
title: 'Docs: TKT-A0RTC52'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: the placement rule in `placeDecisions` and `buttonTypesOf`, the move-gap hold of `buttonTypes`, the focus-follow watches in `OpenChannel.vue`, the cause guard in `InboxWaitingItem.vue`, the button-type selection rule in the `RlChannelComposer` header comment
- [x] Function/type docs if public API: `RlChannel.focusDecisions`, the `buttonTypes` props of `RlChannel` and `RlChannelComposer`, `PlacedDecisions`, `DecisionComposer.replying`

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## User-facing Documentation

- [x] `GUIDE-realtime.md` (generated to `docs/realtime.md` with `just docs`): controls table (decision button, Go to decision), composer paragraph, "Decision buttons", "Preset buttons", Choices and Other answer
- [x] `GUIDE-data-entry.md` (generated to `docs/data-entry.md`): one sentence in "Answering from the inbox"

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no API change)
