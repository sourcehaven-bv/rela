---
id: DOCS-B236R8
type: docs-checklist
title: 'Docs: Soft delete with restore for the Undo toast'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not describe delete behavior)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the project keeps no changelog file)
- [x] API docs updated (if applicable)

The data-entry guide describes undoing a delete and the restore endpoint. The
audit-log, postgres and sqlite guides describe the new audit operations and
tables.
