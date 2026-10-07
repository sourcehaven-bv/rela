---
id: DOCS-YI8FJG
type: docs-checklist
title: 'Docs: Kanban columns from a single-valued relation (columns_from)'
status: done
started: '2026-10-07'
completed: '2026-10-07'
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious
- [x] Function/type docs if public API: Go doc comments on the `ColumnsFrom` config type and `validateKanbanColumnsFrom`/`validateRelationColumns`; TS doc comments in useRelationColumns.ts and styleColors.ts

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change; the demo has its own examples/relation-status-demo/README.md)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] ~~Help text accurate (if CLI changes)~~ (N/A: no CLI changes)

## User-facing Documentation

- [x] `GUIDE-data-entry.md`: section "Columns from a relation (`columns_from`)" with `offered_by`, `order_by`, the Other column, per-column create and `style_from`. Regenerated into `docs/data-entry.md`.

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: no new endpoint; the documented API behaviour is in GUIDE-concepts)
