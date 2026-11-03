---
id: DOCS-F6F2MW
type: docs-checklist
title: 'Docs: Relation fields in view properties sections'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API: Go doc comments on the relation section field config, its validation and the served relation field in sections.go and responses.go; TS doc comments in InlineRelationValue.vue

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the demo has its own examples/relation-status-demo/README.md)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: section "Relation fields" with `fields: - relation:`, the validation rules and `style_from`. Regenerated into `docs/data-entry.md`.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; the documented API behaviour is in GUIDE-concepts)
