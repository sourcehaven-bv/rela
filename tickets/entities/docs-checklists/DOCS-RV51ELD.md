---
id: DOCS-RV51ELD
type: docs-checklist
title: 'Docs: $user form default and list filter'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (resolveFilterVariable on the unidentified caller; the late /_me watch in DynamicForm)
- [x] Function/type docs if public API (resolveFormDefault, USER_TOKEN, filterUser, gatedListQuery.user)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-data-entry.md: the form field default row and a $user example; the filter variable table with $user and its scope; regenerated docs/data-entry.md)
