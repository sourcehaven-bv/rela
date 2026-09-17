---
id: DOCS-EY25Z8
type: docs-checklist
title: 'Docs: per-level, per-type columns for nested sections'
status: done
---

## Code docs

- [x] Godoc on every new symbol: `ViewSection.ParentColumns` / `.ChildColumns`
(one doc block covering both, since the pair only makes sense together),
`collectionTypes`, `validateLevelColumns`, `nestedOnlyKeyErr`,
`nestedColumnUnion`, `columnKey`, `sortedColumnTypes`, `indexed`,
`SectionTreeNode.Columns`, `v1.ViewTreeNode.Columns`.
- [x] Comments state WHY. The load-bearing ones: why level is the primary axis
and type the secondary (a self-referential containment puts one type at both
levels), why `collectionTypes` exists beside `determineTargetType` rather than
replacing it, and why relation columns resolve over the union rather than per
(level, type).
- [x] `just comment-lint` gate clean — no unresolvable doc links across 14,084
comments.

## Project docs

- [x] `docs/data-entry.md` — `parent_columns` and `child_columns` added to the
section fields reference table, and `columns` re-described as `table`-only.
- [x] A new **"Columns are per level, then per type"** subsection under
`#### nested`, giving the worked heterogeneous example (`task` showing `due`,
`bug` showing `severity`) and stating both reasons the two axes exist.
- [x] Documented that a type absent from its level's map renders title and id
only, so adding a type to a relation never breaks an existing view.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/cli-reference.md`~~ (N/A: no CLI change)
- [x] ~~`CLAUDE.md`~~ (N/A: follows the existing `Gantt.Sources` precedent
rather than establishing a new rule)

## External docs

- [x] ~~README~~ (N/A: not a project-level change)
- [x] ~~Changelog~~ (N/A: none maintained in-tree)

## Note

Shipped in the same PR as TKT-MJKZQ3 (#1579), so `docs/data-entry.md` reads as
one coherent description of `display: nested` rather than a base description
plus an amendment. DOCS-NQS6CB covers the base feature's docs; this covers the
column model.
