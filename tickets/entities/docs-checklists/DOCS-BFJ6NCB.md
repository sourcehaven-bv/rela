---
id: DOCS-BFJ6NCB
type: docs-checklist
title: 'Docs: Scoped elevated updates for data-entry actions'
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: reuses the bypass_acl and PatchEntity patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PR titles)
- [x] API docs updated (if applicable)

Data-entry guide: "Elevated updates" section, Action Fields rows and
Validation Rules (`docs-project/entities/guides/GUIDE-data-entry.md`). Lua
guide: "Elevated updates in a data-entry action"
(`docs-project/entities/guides/GUIDE-lua-scripting.md`). Both regenerated into
`docs/`.
