---
id: RES-8CKUNJ
type: research
title: 'Relation-backed status: how should boards and views use a single-valued status relation?'
summary: Status as an entity with typed relation paths (has_status.kleur) read at request time; boards and lists take columns from the relation. Copying status properties onto the task was rejected.
status: in-progress
---

## Problem

A board per parent (initiative, project) needs its own ordered set of columns.
In rela, kanban columns come from one enum per entity type (`column_property`),
so every parent shares one global set. TKT-ZNFGNJ proposes making a task's
status a single-valued relation to a `status` entity. This research decides how
that model interacts with boards and the other views, and which rela
capabilities it needs.

## Context

Survey of develop at `2f7579199`.

- Every enum-driven view feature reads `entity.properties[...]`. That covers
kanban columns and drag (`KanbanView.vue:626-645`), list `group_by`
(`useListGrouping.ts:96-153`), sort (`SortSpec{property}`), `styles:`, page
`badge:`, state machines (`internal/statemachine`), CalDAV completion, dashboard
breakdown, search syntax and next-action `key_props`.
- Relations are supported for display (kanban card fields, TKT-NC3D08; list
columns), as filter controls (TKT-5U7QBR) and in `related()` predicates
(TKT-205V2N).
- Relation filter controls match on the neighbour's **display title**
(`api_v1.go:566-640`). Two statuses both titled "Done" cannot be told apart.
- `max_outgoing` is analysis-only (`internal/schema/cardinality.go:216`).
Nothing stops a task from getting a second status edge.
- A relations PATCH applies add/remove deltas one edge at a time. A
write-loop error leaves earlier edges written: the "documented atomicity gap"
(`relations_modern.go:336`). A re-point is therefore remove plus add, not one
operation.
- Orderable relations exist (`_order_out` / `_order_in`, auto-assigned in
`entitymanager/manager_order.go`). Only the form cards widget uses them, and
`docs/` does not document them (FEAT-FE5P).
- Computed properties cannot read relations (`computed.go:119-122`).
Automations can `set` a property on `relation_created`, but nothing updates the
tasks when the status entity itself changes.
- Prior art: the future concept `extended-property-system` (validated) lists
"property inheritance from related entities" next to rollups (IDEA-001).
- `related(entity, path, {constraints})` (FEAT-RELTRV, RES-RELTRV) gives
EXISTS predicates with SQL pushdown. Value access (`related(...).prop`) was
rejected there for two reasons: host functions may not return records (RR-93UN),
and a third of the relations are multi-target, so a property through them has no
single type. Neither reason applies to a relation that is single-valued
(`max_outgoing: 1`) and single-target (`to: [status]`).
- `RelationPicker` offers every entity of the target type. `FormRelation`
has no Go `default` key, although the SPA reads one.

## Options

### A. Per-parent enabled values of a larger global enum

Keep `status` an enum. Each parent stores which values it shows as columns (a
multi-valued property or a config entry), and the board hides the others.

- Pros: smallest change. Every existing feature keeps working. Sorting and
colour come from the enum.
- Cons: no metadata per status (description, WIP limit). Adding a status
is a schema change and a deploy. The "enabled" list is UI-only, so nothing stops
a task from holding a value its parent does not show.
- Effort: S to M (a kanban `columns_from_property` on the anchor).

### B. Status entity, each feature made relation-aware separately

Status becomes an entity. Each feature in the Context list gains a
relation-backed variant: `group_by` relation, sort by related property, colour
from target, transitions on re-target, relation predicates in search, and so on.

- Pros: one model, no duplicated data.
- Cons: about ten separate feature changes across Go and the SPA, each
needing a join at read time. Sorting and filtering by a related property need
SQL pushdown on postgres (FEAT-R012DX) or they degrade to in-memory work.
- Effort: XL.

### C. Status entity plus materialised lookup properties (rejected)

Copy selected properties of the status (for example `categorie`) onto the task
and keep them in sync, so that property-driven features keep working unchanged.

Rejected. A status carries more than one value (name, colour, order, category,
description, WIP limit), and views need several of them at once. Copying a
subset duplicates data and still leaves the rest unreachable. Compatibility with
existing property-based logic (`status == 'gereed'`) is not a goal: config that
uses the new model is written for the new model.

### D. Status entity plus relation paths (recommended)

Status becomes an entity. rela gains one generic, read-time primitive: a
**relation path** `<relation>.<property>`, allowed only when the relation is
single-valued (`max_outgoing: 1`) and single-target. The path is typed
statically from the target property, which avoids the soundness problems
RES-RELTRV found for multi-target relations.

A path is accepted wherever view config takes a property name:

```yaml
sort:
  - property: has_status.volgorde
group_by:
  relation: has_status            # sections are the target entities
card:
  fields:
    - property: has_status.titel
styles_from: has_status.kleur     # badge colour from the target
```

The server resolves a path with a join to the target, pushed down to SQL on
postgres in the same way as `related()`. The relation-backed board and list
build on the same resolution:

1. **Kanban columns from a relation.** A kanban `columns_from` block names
the relation (`has_status`) and, optionally, the anchor relation that offers and
orders the columns (`offers_status`, in `_order_out` order). Without an anchor,
the columns are all targets, ordered by a path.
2. **Drag re-targets the relation** with one `replace` operation.
3. **Per-column create** prefills the relation (`rel.has_status=<id>`).
4. **"Other" column** for tasks whose status the anchor does not offer,
shown only when it is not empty.
5. **List `group_by` relation**, with the same column resolution.
6. **Scoped picker candidates and a working relation `default`.**
7. **Relation filter controls by id**, not title.

Places that today need a property value read the path instead: CalDAV completion
(`has_status.categorie`), dashboard breakdown, search `prop:`, gantt tooltip
fields and next-action `key_props`.

- Pros: no duplicated data. Every property of the status is reachable. One
typed primitive replaces the per-feature work of option B.
- Cons: every read that uses a path costs a join. ACL applies to the target:
when a reader may not read the status entity, the path resolves to empty, and
grouping puts the task under "Other" for that reader.
- Effort: L in total, in independent tickets.

## Recommendation

Option D.

Settled within option D:

- **Global status set, per-parent selection.** Statuses are global entities.
A parent selects and orders them through an orderable relation. A template is a
preset of those edges. Private statuses per parent are out of scope, because a
global `volgorde` keeps sorting meaningful across parents.
- **No compatibility layer.** Config that adopts relation-backed status reads
the relation or a path. Nothing keeps an enum `status` in sync.
- **A re-point is one `replace` operation.** With write-time enforcement of
`max_outgoing: 1`, a second `add` is rejected rather than silently kept.
- **Column order:** the anchor's `_order_out` when an anchor relation is
configured, otherwise a path on the target.

Open:

- **Transitions.** State machines are defined on enum types. For status
entities, allowed moves could be edges between statuses (for example `status
--may_move_to--> status`), checked on `replace`. Alternatively there are no
transitions in the first iteration.
- **Paths in conditions.** `entity.has_status.categorie == 'gereed'` would be
sound for single-valued single-target relations. `related()` already covers it
as a predicate, so this is not needed for the first iteration.

Follow-up tickets, in dependency order:

1. TKT-65LVAK: write-time `max_outgoing` enforcement plus a `replace`
operation.
2. TKT-DA9C0L: relation paths in view config.
3. TKT-KJ3Q07: kanban `columns_from` relation (drag, per-column create,
Other column).
4. TKT-JO8PN3: list `group_by` on a relation.
5. TKT-2EN0G5: scoped relation picker candidates and `FormRelation.default`.
6. TKT-ZKPA1E: relation filter controls by id.

Colour from the status entity uses `has_status.kleur`. A real colour property
type (TKT-28FRME) is needed for free colours; until then `kleur` is an enum
mapped through `styles:`.
