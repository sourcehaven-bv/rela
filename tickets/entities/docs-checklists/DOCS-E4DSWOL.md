---
id: DOCS-E4DSWOL
type: docs-checklist
title: 'Docs: Create a column from the board'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (validateKanbanAddColumn and addEnumColumn explain why only an inline enum is edited; KanbanAddColumn explains the save gate and reload)
- [x] Function/type docs if public API (Kanban.AddColumn and KanbanConfig.add_column)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README content on boards)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: uses the existing Configure write path)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file; release notes come from PRs)
- [x] API docs updated (if applicable) (docs-project data-entry guide: Kanban Fields table and the section Adding a column from the board; docs/ regenerated with just docs)
