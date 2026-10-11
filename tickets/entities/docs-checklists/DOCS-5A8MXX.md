---
id: DOCS-5A8MXX
type: docs-checklist
title: 'Docs: Version tags'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: README does not cover history)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: tags follow the existing versioning and purge rules)
- [x] Help text accurate (if CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo keeps no changelog)
- [x] API docs updated (if applicable)

Guides updated: postgres backend (version tags section), CLI reference
(history-tag, --force-tags), Lua scripting (tag functions, per-reader tokens),
ACL security (`tag:<ns>`); docs/ regenerated.
