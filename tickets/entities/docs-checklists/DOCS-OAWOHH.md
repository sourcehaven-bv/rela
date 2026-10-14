---
id: DOCS-OAWOHH
type: docs-checklist
title: 'Docs: Lua rela.xml and rela.feed'
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (safety argument in the safexml package doc; ElementTree model; date zone handling; item cap rationale)
- [x] Function/type docs if public API (safexml, feed and the Lua module headers document every exported symbol and the Lua table shapes)

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog; release notes come from PR titles)
- [x] API docs updated (GUIDE-lua-scripting: new "XML Functions" and "Feed Functions" sections; docs/lua-scripting.md regenerated)
