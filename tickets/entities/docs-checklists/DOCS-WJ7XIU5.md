---
id: DOCS-WJ7XIU5
type: docs-checklist
title: 'Docs: Dashboard cards render values like the rest of the app'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (compareGroups on why it mirrors the server's groupCounts; per-row type resolution in queryBreakdown and tableRows; widget caching in createDenseCellResolver)
- [x] Function/type docs if public API (CardSchema, BreakdownItem, TableCell, queryBreakdown, listBreakdown, tableRows, createDenseCellResolver, styleBarColor)

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not cover dashboard cards)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; release notes come from PR titles)
- [x] API docs updated (GUIDE-data-entry and generated docs/data-entry.md: list-form `sort:` in the dashboard, list and full examples; card option `sort` documented as a list; breakdown labels, colours and order; table cell rendering and the `width:` hint)
