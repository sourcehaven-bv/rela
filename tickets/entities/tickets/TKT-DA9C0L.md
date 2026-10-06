---
id: TKT-DA9C0L
type: ticket
title: 'Lookup properties: copy a property from a single-valued relation target'
kind: enhancement
priority: high
effort: l
status: backlog
---

## Description

Add **lookup properties**: a read-only property whose value is copied from a
property of the target of a single-valued relation, stored and indexed like any
other property. Part of RES-8CKUNJ.

```yaml
taak:
  properties:
    status_categorie:
      type: lookup
      relation: has_status   # must have max_outgoing: 1
      property: categorie    # type is inherited from the target property
```

Because the value is stored on the source entity, every property-driven feature
keeps working unchanged: `styles:`, state machines, CalDAV completion,
`condition:`, search `prop:`, sort, dashboard breakdown, next-action
`key_props`, validations and scripts.

This extends FEAT-IB6S20 (computed properties over the same entity) to one
relation hop. It is the narrowest form of "property inheritance from related
entities" in the `extended-property-system` future concept.

## Scope

- Schema load validates that the relation is single-valued (depends on the
write-time cardinality ticket) and that the target property exists.
- Recompute on: relation add, remove and `replace` on the source; and an
update of the looked-up property on any target, which rewrites every source
entity in the same transaction.
- Lookups are never user-editable on any write path.
- ACL: decide whether a lookup is visible only when the reader may read the
source property on the target, or whether lookups are restricted to targets
every reader of the source may read. Do not leak target data by copying it.
- `rela analyze` (or `rela migrate`) detects and repairs drift in stores edited
outside rela.

## Acceptance criteria

- Changing a task's status updates its lookup values in the same write.
- Changing a status entity's `categorie` updates all linked tasks.
- A lookup can be used as `column_property`, `group_by`, sort key, style key
and state-machine property, with no other config change.
