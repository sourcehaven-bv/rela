---
id: TKT-DA9C0L
type: ticket
title: 'Relation paths: read a property of a single-valued relation target in view config'
kind: enhancement
priority: high
effort: l
status: backlog
---

## Description

Add **relation paths**: `<relation>.<property>` as a value reference in view
config, allowed only when the relation is single-valued (`max_outgoing: 1`)
and single-target. Part of RES-8CKUNJ (option D).

The path is typed statically from the target property. RES-RELTRV rejected
value access through relations because multi-target relations have no single
property type and host functions may not return records. A single-valued,
single-target relation has neither problem, and the path lives in view config,
not in the condition language.

```yaml
sort:
  - property: has_status.volgorde
card:
  fields:
    - property: has_status.titel
columns:
  - property: has_status.categorie
```

Values are read at request time through a join to the target. Nothing is
copied onto the source entity: a status has several properties (name, colour,
order, category, description) and views need several of them at once.

## Scope

- Config load accepts a path wherever a property name is accepted in: list
  columns, `sort` / `default_sort`, kanban card fields, view section fields,
  gantt tooltip fields, calendar event fields, dashboard `group_by`, CalDAV
  `completion.status_property` and next-action `key_props`. Load rejects a
  path over a multi-valued or multi-target relation, and an unknown target
  property.
- Badge colour from a path (`styles_from: has_status.kleur` or similar) for
  enum-typed target properties.
- Search syntax: `prop:has_status.categorie=gereed`.
- Postgres: sort and filter on a path push down as a join. Count queries per
  request stay bounded (FEAT-R012DX).
- ACL: the target is read through the reader's visibility. A hidden target
  resolves to empty.
- Out of scope: paths in the Lua condition language (`related()` covers
  predicates), and paths of more than one hop.

## Acceptance criteria

- A task list sorts by `has_status.volgorde` and shows `has_status.titel` as a
  column, on both file and postgres backends.
- Renaming a status changes what every view shows, with no write to tasks.
- A path over a relation without `max_outgoing: 1` fails config load with a
  message naming the relation.
