---
id: DOCS-T9XEP2
type: docs-checklist
title: 'Docs: relation order reorder'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (PlaceOrder densify rule, slotValue threshold, move visibility filter, createMoveQueue settle timing, section copy-before-sort)
- [x] Function/type docs if public API (entity.OrderPosition incl. Among, metamodel.OrderKey/CompareOrderKeys, v1.RelationOrder, RlBoard `reorder` prop and `move.at`, useDropTargetCard)

## Project Documentation

- [x] ~~README updated~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; the visible-only move rule is documented in docs/data-entry/api-reference.md and the godoc on OrderPosition)
- [x] ~~Help text accurate~~ (N/A: no CLI changes)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: no changelog file in this repo)
- [x] API docs updated (docs/data-entry/api-reference.md: "Moving an edge (`position`)" with answers table, visible-only move rule, faced anchor addressing, and "Reading the order"; docs/metamodel.md: "Ordered Relations (`orderable:`)"; docs/data-entry.md: "Rows in relation order")
