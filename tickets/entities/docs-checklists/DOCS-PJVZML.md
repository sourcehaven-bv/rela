---
id: DOCS-PJVZML
type: docs-checklist
title: 'Documentation: Collapsible kanban columns'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

`KanbanColumn.Collapsed` godoc says it is a default only. `useKanbanCollapse`
documents why overrides (not a set) are stored, why a default-equal override is
dropped, and that tabs do not sync. `useSectionToggleFocus` documents why focus
must be moved after each toggle. The new library props (`collapsible`) and emits
(`collapseSection`) carry doc comments.

## Project Documentation

- [x] README updated (if applicable)
- [x] CLAUDE.md updated (if new patterns)
- [x] Help text accurate (if CLI changes)

None applicable: no CLI change and no new project-wide pattern.

## External Documentation

- [x] Changelog entry added
- [x] API docs updated (if applicable)

`docs-project/entities/guides/GUIDE-data-entry.md` (source of the generated
`docs/data-entry.md`, regenerated with `just docs`): the Kanban Fields table
lists `collapsed`, the Columns table gains `icon` and `collapsed`, and a new
"Collapsing columns" section explains the heading control, per-browser memory,
the `collapsed: true` default and that cards cannot be dropped on a collapsed
column. The repo has no CHANGELOG; release notes come from commits. No API
change beyond the additive `collapsed` field on kanban columns in `_config`.
