---
id: TKT-RTH3
type: ticket
title: Add task list (checkbox) support to Lua markdown AST
kind: enhancement
priority: medium
effort: m
status: done
---

# Add Task List (Checkbox) Support to Lua Markdown AST

Extend the Lua `rela.md` module to parse, represent, and serialize task list
items (`- [x]` / `- [ ]`). Currently list items are plain strings with no
checkbox state, making it impossible to script around checklists.

## Motivation

Entity content often contains task lists (planning checklists, implementation
checklists, etc.). Lua scripts should be able to:

- Parse task list items and read their checked/unchecked state
- Modify checkbox state programmatically
- Create new task list items with checkbox state
- Serialize task lists back to valid markdown

This enables scripting workflows like bulk-checking items, generating progress
reports, or transforming checklists.

## Scope

**In scope:**

- Enable goldmark `TaskList` extension in `luaMdParse`
- Extend list item representation from plain string to table with `text`,
`checked` (boolean), and `task` (boolean) fields
- Update `renderList` to serialize checkbox syntax
- Update `luaMdList` constructor to accept task items
- Add helper functions for common task operations

**Out of scope:**

- Nested task lists (keep flat like current list support)
- Strikethrough detection on task items (handled by checklist validation)

## Acceptance Criteria

1. `rela.md.parse()` correctly parses `- [x] done` and `- [ ] todo` into
AST nodes with `checked` and `task` fields
2. `rela.md.render()` serializes task items back with correct checkbox syntax
3. `rela.md.list()` constructor accepts task item tables
4. Mixed lists (some items with checkboxes, some without) round-trip correctly
5. Existing scripts using plain string list items continue to work (backward
compatible)
