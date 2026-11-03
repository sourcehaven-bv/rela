---
id: DOCS-WDKF58
type: docs-checklist
title: 'Docs: Show a related entity''s property on a kanban card'
status: done
---

## Code Documentation

- [x] Comments where logic isn't obvious (relatedCardValues on why no raw-id fallback and how gating applies; checkCardFieldRelatedProperty on why every target type must declare the property; renameCardFields on which type owns the property)
- [x] Function/type docs if public API (KanbanCardField, RelatedCardValue, ResolvedCardField.values, refuseKanbanOnlyFields)

## Project Documentation

- [x] ~~README updated~~ (N/A: README does not cover kanban keys)
- [x] ~~CLAUDE.md updated~~ (N/A: no new pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; release notes come from PR titles)
- [x] API docs updated (GUIDE-data-entry and generated docs/data-entry.md: "Properties of related entities" under kanban card configuration; direction table note)
