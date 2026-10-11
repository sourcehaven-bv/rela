---
id: DOCS-4E0PNJ
type: docs-checklist
title: 'Docs: External-ref property type and Lua 3-way merge helper'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not cover property types or sync)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: ref write gate follows the existing computed-property refusal pattern)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog)
- [x] API docs updated (if applicable)

Guides updated: metamodel (external_ref type, sync opt-in, uniqueness), Lua
scripting (find_by_external_ref, rela.sync.merge, EMPTY, expect and write
tokens, reference sync loop), CLI reference (analyze duplicate refs, scheduler
refusal); docs/ regenerated.
