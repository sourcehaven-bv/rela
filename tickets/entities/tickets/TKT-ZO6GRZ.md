---
id: TKT-ZO6GRZ
type: ticket
title: 'Read-only graph access in Lua migration steps: rela.get_entity and rela.get_relations'
kind: enhancement
priority: medium
effort: m
started: "2026-10-13"
completed: "2026-10-13"
status: done
---

## Description

A `lua:` migration step (`function migrate(entity)`) receives only `{id, type,
content, properties}`. It cannot read relations or other entities, so a
migration cannot classify an entity by its place in the graph.

Atlas needed exactly that. Its `taak` type gained a required `soort` enum, and
existing tasks had to be classified: a task whose `gaat_over` relation points to
an `ncr` becomes `corrective_action`. A CISO review required the classification
to be done by the migration, not by hand. This kind of requirement will recur.

This ticket adds read-only graph access to the migration sandbox:

- `rela.get_entity(addr)`: one entity, or nil.
- `rela.get_relations({from=, to=, type=})`: matching relations.

The signatures follow the normal scripting API (docs/lua-scripting.md).

## Requirements

1. The script stays a pure transform. It returns a patch for the entity it
was handed. No write function is exposed.
2. Reads are consistent and do not depend on iteration order. Every read in a
step sees the store as it was before the step. Entity B never sees the patch the
same step made to entity A. A later step sees the results of earlier steps.
3. Re-runs stay idempotent.
4. Reads bypass validation and ACL, like the rest of the runner. Data may be
invalid under the new schema mid-migration.
5. Works on every backend that runs migrations (fs, sqlite, postgres) and for
`entity: "*"`.

## Out of scope

- `rela.list_entities`, search and trace inside migrations. They can be
added later on the same reader if a migration needs them.
- A richer dry-run report: see the follow-up ticket linked from here.
