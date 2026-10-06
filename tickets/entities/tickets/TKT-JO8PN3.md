---
id: TKT-JO8PN3
type: ticket
title: List group_by on a single-valued relation
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Let list `group_by` group on a single-valued relation. Sections are the target
entities, with their order and label taken from the target. Part of RES-8CKUNJ.

```yaml
group_by:
  relation: has_status
  offered_by: offers_status   # optional, same meaning as on the kanban
  order_by: volgorde
```

## Scope

- Reuse the column resolution from the kanban `columns_from` ticket so lists
and boards agree on order and on the "Other" section.
- The section create prefill sets the relation instead of a property.
- Empty sections follow the existing `groups` behaviour.

## Acceptance criteria

- A grouped list and the kanban for the same anchor show the same sections
in the same order.
- Creating from a section links the new entity to that section's target.

## Demo

Branch `demo/relation-backed-status`: `group_by: {relation, offered_by,
order_by}` with validation shared with the kanban (`validateRelationColumns`).
Sections come from `useRelationColumns`; Add from a section prefills the
relation. Demo lists: `taken` and the `lijst` tab on the initiative page.
