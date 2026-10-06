---
id: RES-8CKUNJ
type: research
title: 'Relation-backed status: how should boards and views use a single-valued status relation?'
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
  Option C below is the narrowest form of that inheritance.
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

### B. Status entity, every feature made relation-aware

Status becomes an entity. Each feature in the Context list gains a
relation-backed variant: `group_by` relation, sort by related property, colour
from target, transitions on re-target, relation predicates in search, and so on.

- Pros: one model, no duplicated data.
- Cons: about ten separate feature changes across Go and the SPA, each
needing a join at read time. Sorting and filtering by a related property need
SQL pushdown on postgres (FEAT-R012DX) or they degrade to in-memory work.
- Effort: XL.

### C. Status entity plus materialised lookup properties (recommended)

Status becomes an entity, as in B. rela gains one generic metamodel feature, a
**lookup property**: a read-only property whose value is copied from the target
of a single-valued relation and kept in sync on write.

```yaml
taak:
  properties:
    status_categorie:
      type: lookup
      relation: has_status       # must be max_outgoing: 1
      property: categorie        # enum on the status type; type is inherited
    status_volgorde:
      type: lookup
      relation: has_status
      property: volgorde
```

Because the value is stored on the task, every property-driven feature works
unchanged: `styles:`, state machines, CalDAV completion, `condition:`, search
`prop:`, sort, dashboard breakdown, `key_props`, validations and scripts.

Only the features that must see the status *entity* need new work:

1. **Kanban columns from a relation.** A kanban `columns_from` block names
the relation (`has_status`) and, optionally, the anchor relation that offers and
orders the columns (`offers_status`, read in `_order_out` order). Without an
anchor, columns are all targets ordered by a target property.
2. **Drag re-targets the relation.** A drop sends one `replace` operation
for the single-valued relation instead of a properties PATCH.
3. **Per-column create** prefills the relation (`rel.has_status=<id>`, which
already works through query params).
4. **"Other" column.** Tasks whose status the anchor does not offer appear
in a trailing column, shown only when it is not empty.
5. **List `group_by` relation**, with section order and label taken from
the target. This is the same mechanism as point 1.
6. **Scoped picker candidates and a working relation `default`**, so the
form offers only the anchor's statuses and preselects the first.
7. **Relation filter controls by id**, not title.

Write-time enforcement of `max_outgoing: 1` and an atomic `replace` operation
are prerequisites for both the lookup and the board.

- Pros: most of rela keeps working with no changes. The new surface is
small and generic (lookup properties also help other models, such as an owner's
department). Postgres can index and sort lookup values like any property.
- Cons: write amplification. Editing a status entity's `categorie` rewrites
every linked task, in one transaction on the status write. Lookup values are
denormalised, so a store edited outside rela (a file-backed project edited by
hand) can drift until `rela migrate` or `analyze` repairs it. A lookup also
copies data across an ACL boundary: a reader who may see the task but not the
status entity still sees the copied value. Either a lookup is visible only
when the source property is visible to that reader, or lookups are limited to
targets that every reader of the source may read.
- Effort: L in total, in independent tickets.

## Recommendation

Option C. It turns roughly ten view changes into one metamodel feature and a
handful of kanban and list changes. Option A remains a fallback when per-status
metadata is not needed.

Settled within option C:

- **Global status set, per-parent selection.** Statuses are global entities.
A parent selects and orders them through an orderable relation. A template is a
preset of those edges. Private statuses per parent are out of scope, because a
global `volgorde` keeps sorting meaningful across parents.
- **Category drives behaviour.** State machines, CalDAV, validations and
scripts read the lookup `categorie` (open, active, waiting, done). They never
read the status entity's title.
- **Transitions stay on the category.** No transition model for relation
re-targeting in the first iteration.
- **A re-point is one `replace` operation.** With write-time enforcement of
`max_outgoing: 1`, a second `add` is rejected rather than silently kept.
- **Column order:** the anchor's `_order_out` when an anchor relation is
configured, otherwise the target's order property.

Follow-up tickets, in dependency order:

1. Write-time `max_outgoing` enforcement plus a `replace` operation.
2. Lookup properties.
3. Kanban `columns_from` relation (drag, per-column create, Other column).
4. List `group_by` on a relation.
5. Scoped relation picker candidates and `FormRelation.default`.
6. Relation filter controls by id.

Colour from the status entity depends on a colour property type (TKT-28FRME).
Until then, a lookup of an enum `kleur` property with `styles:` covers it.
