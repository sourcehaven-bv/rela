---
id: TKT-CADCFX
type: ticket
title: Relation fields in view properties sections
kind: enhancement
priority: medium
effort: m
status: done
started: '2026-10-06'
completed: '2026-10-07'
---

Let a view `properties` section on the entry show a relation as a field among
the properties, with `fields: - relation: <name>`. The field shows the target
titles and changes them in place with a compact picker. A single-valued relation
(`max_outgoing: 1`) re-points in one PATCH; a multi-valued one adds and removes.

Today every relation gets its own cards section on the detail page, which is
heavy for a single-valued status, owner or initiative. Part of RES-8CKUNJ.

```yaml
sections:
  - source: entry
    display: properties
    render: input
    fields:
      - property: titel
      - relation: heeft_status
        label: Status
```

Validation: one of property or relation; relation fields only in `source: entry`
+ `display: properties`; the relation must exist and start at the entry type;
`widget:` does not apply.

A follow-up could show single-valued relations this way in the default view
without config.

## Demo

Branch `demo/relation-backed-status`: the demo's `taak` view shows Status as a
field next to Deadline. Candidates are listed by title; an `order_by` on the
field (as on `columns_from`) would list statuses in their own order.

`style_from: <target enum property>` shows each target as a badge with its
title, coloured by that property's `styles`. The demo colours statuses by
`categorie`.
