---
id: DOCS-CMOM3EH
type: docs-checklist
title: 'Docs: Lua rela.attach_file'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: follows the attach_render pattern)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog file; release notes come from PRs)
- [x] API docs updated (if applicable)

docs/lua-scripting.md gets a `rela.attach_file` section;
docs/attachment-security.md lists it among the ingresses that run the attachment
processor. Both generated from docs-project.
