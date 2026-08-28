---
id: DOCS-8HD
type: docs-checklist
title: Documentation
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated~~ (N/A)
- [x] ~~CLAUDE.md~~ (N/A: no new pattern)
- [x] ~~Help text accurate~~ (N/A: no CLI surface change)

## External Documentation

- [x] ~~Changelog entry~~ (N/A: repo keeps no CHANGELOG)
- [x] docs/acl-overview.md — 'Cascade delete needs the relation grants too' documents the behaviour change (an entity delete can now fail on a relation grant), the per-source-type check, and that the check runs inside the delete transaction
