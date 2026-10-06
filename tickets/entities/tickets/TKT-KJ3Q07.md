---
id: TKT-KJ3Q07
type: ticket
title: Kanban columns from a single-valued relation (columns_from)
kind: enhancement
priority: high
effort: l
status: backlog
---

## Description

Let a kanban take its columns from a single-valued relation instead of an enum
property. Part of RES-8CKUNJ.

```yaml
kanbans:
  initiatief_bord:
    entity_type: taak
    columns_from:
      relation: has_status        # max_outgoing: 1 on the card type
      offered_by: offers_status   # optional: relation from the page anchor
      order_by: volgorde          # used when offered_by is absent
```

## Scope

- Columns are the target entities. With `offered_by`, they are the anchor's
targets in `_order_out` order (orderable relation, FEAT-FE5P). Without it, all
targets ordered by `order_by`.
- A drop sends one `replace` operation on the relation (write-time
cardinality ticket) with an optimistic update on `relations`, not a properties
PATCH.
- Per-column create opens the create form with `rel.<relation>=<id>`
prefilled, plus the anchor link `scope` already adds.
- Cards whose target is not offered by the anchor, or that have no target,
appear in a trailing "Other" column that is shown only when it is not empty. No
card is hidden silently.
- Column header label and colour from relation paths on the target
  (`has_status.titel`, `has_status.kleur`, TKT-DA9C0L).
- Swimlanes from a relation are out of scope.

## Acceptance criteria

- An initiative page with an `offers_status` order shows exactly those
columns in that order, plus "Other" when needed.
- Dragging a card changes its status edge and nothing else.
- Reordering the anchor's `offers_status` reorders the columns.
