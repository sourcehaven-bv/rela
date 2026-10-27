---
id: DOCS-X2RG7Z
type: docs-checklist
title: 'Docs: rela.tagged_entities (TKT-7LRUPB)'
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (shared SQL CTE steps, deleted-row gate rule, row type choice, ordinal 0 case)
- [x] Function/type docs if public API (store.TaggedEntity, store.VersionTagIndex, ScriptReader.TaggedEntities, VersionTagReader, WithHistory)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the existing history-read and collection-read rules)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-lua-scripting: function table row and Baseline and compare section; GUIDE-acl-security: listing by version tag; docs/ regenerated)
