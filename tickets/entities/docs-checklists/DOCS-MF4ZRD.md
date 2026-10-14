---
id: DOCS-MF4ZRD
type: docs-checklist
title: 'Docs: Lua code sharing via require (TKT-PO53JO)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (why require is rela-owned, thread per module top level, pendingFrame, migration steps keep require nil, module files skip non-.lua under lib/ on sqlite)
- [x] Function/type docs if public API (lua.ModuleDir, Runtime.RunChunk, lua.FailedInModule, ScriptError.LuaPath)

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not describe script layout)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting rule; the feature is documented in GUIDE-lua-scripting)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI flags or commands changed)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog)
- [x] API docs updated (if applicable) (GUIDE-lua-scripting: new "Sharing code between scripts" section and sandbox lists; GUIDE-data-migration: no require in steps; GUIDE-sqlite-backend: lib/*.lua stored in rela.db; docs/ regenerated with just docs)
