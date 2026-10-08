---
id: TKT-RCRUWZ
type: ticket
title: Drag-and-drop reorder of a list scoped to one entity through an orderable relation
kind: enhancement
priority: medium
effort: l
started: "2026-10-06"
completed: "2026-10-06"
status: done
---

## Description

Orderable relations already exist (TKT-XF5F): a relation type declares
`orderable: outgoing|incoming|both`, each edge carries a managed `_order_out` /
`_order_in` value, and the engine appends and renumbers. The only UI that uses
it is drag-to-reorder in the edit form's relation cards.

List views ignore the order. When a list is scoped to one entity (an entity page
tab with `scope: {relation, direction}`, for example a project's tasks), the
rows are sorted by entity properties only and cannot be reordered.

## Goal

On a list scoped to one anchor entity through an orderable relation:

- rows are shown in relation order by default;
- the user can drag a row to a new position, which writes the new order value
to the anchor's edge through the existing relation PATCH endpoint.

## Notes

- `resolvePageScope` (internal/dataentry/pagescope.go) already loads the
anchor's edges but keeps only the ids; it can keep the order values.
- Reordering only makes sense when the list shows relation order: disable drag
while a column sort, search or filter is active.
- `orderable` is undocumented in docs/metamodel.md; document it here.
