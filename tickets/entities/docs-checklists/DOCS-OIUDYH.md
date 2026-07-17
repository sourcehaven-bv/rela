---
id: DOCS-OIUDYH
type: docs-checklist
title: 'Docs: Relation filter_controls render as target selector'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] New component/function doc comments (`EntityTargetSelect.vue` has a full header doc; FilterBar relation helpers commented)
- [x] ~~Public API docs~~ (N/A: no Go/API surface change; frontend-only widget)

## Project Documentation

- [x] `docs/data-entry.md` — Filter Controls section updated: documents `relation`/`direction` fields, the select→typeahead target-selector behavior, and the three known limitations (title collisions, ~100-per-type fetch cap, non-identifier relation names can't be deep-linked).
- [x] ~~docs/metamodel.md~~ (N/A: no metamodel schema change)
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting convention)
- [x] ~~README.md~~ (N/A: no project-level change)

## External Documentation

- [x] ~~Changelog / release notes~~ (N/A: handled at release time; feature is config-transparent — existing relation `filter_controls` auto-upgrade)

**Documentation updated:** `docs/data-entry.md` (Filter Controls section).
