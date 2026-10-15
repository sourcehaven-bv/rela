---
id: DOCS-VZF7RE
type: docs-checklist
title: 'Docs: Kanban column colour, column summaries and currency display'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (columnSummary and columnColor in KanbanView, formatSummary on empty and partial values, validatePropertyDisplay on why it checks shape and not a list)
- [x] Function/type docs if public API (Summary, SummaryFunction, PropertyDisplay, Section.summary, formatSummary, formatDisplayNumber)

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not cover kanban keys)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; release notes come from PR titles)
- [x] API docs updated (GUIDE-data-entry and generated docs/data-entry.md: column_summary in the kanban fields table, "Column summaries" section with the reuse intent, column colour note; GUIDE-metamodel and docs/metamodel.md: `display` option and "Display formats")
