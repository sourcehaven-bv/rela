---
id: DOCS-TC6QD59
type: docs-checklist
title: 'Docs: Bound the remaining Lua read bindings'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: the README does not describe the Lua read bindings)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new pattern; the change follows collection-reads.md)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; the PR body lists the behaviour changes)
- [x] API docs updated (if applicable)

GUIDE-lua-scripting (docs-project, regenerated into docs/lua-scripting.md)
documents `limit` on get_relations, admin.get_relations and
admin.list_entities, the raise on store errors, and the `ids`, `content` and
`limit` options of rela.md.entity_refs.
