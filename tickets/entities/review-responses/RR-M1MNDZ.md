---
id: RR-M1MNDZ
type: review-response
title: Moving the title column down in Configure fails at save only
finding: With the column 0 refusal settled, dragging a busy column to the top, or removing the title column above it, is accepted by the Configure page (ListView.vue:133,157) and refused only when the operator saves. Consider disabling that move in RlSortableList for a busy row, or show the load error next to the column.
severity: nit
resolution: ADDRESSED ListView move handler skips a move that would make a busy column first, Remove on column 0 is disabled when column 1 is busy, and the Columns section explains why; covered by models.test.ts guards and ListView.test.ts
status: addressed
---
