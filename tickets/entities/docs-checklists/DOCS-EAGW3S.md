---
id: DOCS-EAGW3S
type: docs-checklist
title: 'Docs: channel store (TKT-DD8B6I)'
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (segment layout, tail reads, incarnation, tombstone and rekey steps, compaction threshold, migration number)
- [x] Function/type docs if public API (channels.Store and its types, CheckIncarnation, AnchorModeChanges, appbuild.Channels)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the comments and piles storage pattern in .claude/rules/storage.md)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] ~~API docs updated (if applicable)~~ (N/A: no HTTP or MCP surface until T3; storage layout documented in package godoc)
