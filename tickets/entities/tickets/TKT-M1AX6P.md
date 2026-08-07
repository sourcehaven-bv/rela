---
id: TKT-M1AX6P
type: ticket
title: 'Standalone documents: document: as a navigation entry with optional entity_type'
kind: enhancement
priority: medium
effort: m
status: done
---

## Problem

`navigation:` supports `list`, `dashboard`, `kanban`, `search`, `settings` and
`action`, but not `document`. Documents whose content is company-wide rather
than about one entity currently have no natural home:

- `documents:` requires an `entity_type`.
- The route `/document/:name/:entityId` requires an id.

So such a report must be anchored to an arbitrary entity that does not actually
drive its content — the entity is a routing artifact, not a subject.

## Proposal

Allow a navigation entry of the form:

```yaml
navigation:
  - label: 'Verkooprapportage'
    document: sales_review
```

and make `entity_type` optional in `documents:`:

```yaml
documents:
  sales_review:
    title: "Verkooprapportage"
    script: docs/sales_review.lua
```

With no entity type:

- the route becomes `/document/:name` (no entity id segment);
- `rela.document.entry_id` is nil.

Document-mode scripts must already tolerate a nil `entry_id`, since list-render
mode leaves it unset today.

## Use case

A periodic sales report for the directie, aggregated across organisaties,
opportunities and abonnementen. It belongs in the sidebar next to the dashboard,
not on a product page.
