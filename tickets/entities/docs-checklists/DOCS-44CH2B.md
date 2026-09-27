---
id: DOCS-44CH2B
type: docs-checklist
title: 'Docs: Push down query scopes built from related() and equalities'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no README-level change)
- [x] CLAUDE.md updated (if new patterns)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repository keeps no changelog file)
- [x] API docs updated (if applicable)

Updated: the "Filtering on related entities" section of GUIDE-metamodel
(regenerated into docs/metamodel.md) says which scopes a list page answers in
the store and which keep the slower path. CLAUDE.md "Collection reads" states
the exact-lowering rule and that traversals ride in GraphQuery.Related.
