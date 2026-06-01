---
id: TKT-XF5F
type: ticket
title: Reorderable relations via metamodel-declared ordering property
kind: enhancement
priority: medium
effort: l
status: done
---

## Problem

When a user lists related entities through a relation (e.g., `parent
--has-children--> child`) the order is determined by storage/iteration order,
not by user intent. There is no way to express "these related items should
appear in this specific order". Users currently work around this with prefixes
in titles or ad-hoc properties, neither of which the UI or other tooling
understands.

## Proposal

Let the metamodel declare, per relation type, that the relation is
**orderable**, naming a property on the relation that holds the ordering value.
The data-entry UI then renders linked items sorted by that property and offers
drag/move-up/move-down controls that write back to it.

Example metamodel snippet (illustrative, exact shape TBD in planning):

```yaml
relations:
  has-step:
    from: [recipe]
    to: [step]
    properties:
      _order: { type: integer }
    orderable:
      by: _order            # property to sort by
      direction: outgoing   # which side this order applies to
```

## Open questions (for planning)

- **Naming**: is the ordering property always an underscore convention (`_order`) implicitly managed, or any user-named property? Trade-off: underscore = simpler UX, magic; named = explicit, reusable for human-meaningful sort keys.
- **Outgoing vs incoming**: a relation has two sides. The list of children under a parent (outgoing) and the list of parents under a child (incoming) may both want ordering, and they may want different orders. Do we need two ordering properties per relation type, or one with a configured direction, or order the *edge* and pick the side at render time?
- **Polymorphism**: relations can fan out across multiple `from` or `to` types. If a list mixes types under one parent, does ordering remain global across the mixed list? Likely yes, since the order is stored on the relation, not the target — but the UI surface needs to handle a mixed-type sortable list.
- **Storage backing**: integer ordinals require renumber-on-insert (or sparse/float midpoint scheme like LexoRank). Pick a scheme that allows single-edge updates on reorder without rewriting every sibling.
- **Manual edits**: external markdown edits can produce gaps, duplicates, or missing order values. Behavior must be tolerant (sort stably by `(order, fallback)`), per rela's permissive-storage policy.

## Scope

In scope:

- Metamodel schema for declaring orderable relations
- Validation/loader support
- Data-entry UI: render related items in declared order, provide reorder controls
- API/MCP path to persist the order change (likely PATCH on the relation)

Out of scope (this ticket):

- Lua-side reorder helpers
- Bulk reorder operations across many parents
- Free-form numeric edit of the order property in the UI (drag / move-up-down is enough)

## Acceptance criteria (sketch — to be refined in planning)

1. A relation type declared orderable in the metamodel is rendered in the declared order in data-entry list/detail views.
2. Drag-to-reorder in the data-entry UI persists the new order via the API.
3. Reordering a single item requires writes to a bounded number of relations (no full renumber on every move).
4. Missing/duplicate order values do not break rendering — items sort stably.
5. Analyze tools surface duplicate/missing order values as warnings, not hard errors.
