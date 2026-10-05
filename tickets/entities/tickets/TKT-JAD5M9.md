---
id: TKT-JAD5M9
type: ticket
title: rename_relation_type does not refuse a content/identity scope change
kind: enhancement
priority: low
effort: m
status: backlog
description: Record relation scope in ShapeProjection and refuse a rename_relation_type step that changes scope.
---

## Description

Deferred from TKT-KQXVF7 review (RR-W0H7J6). The `rename_relation_type`
data-migration step (`renameRelationTypeStep` in `internal/datamigration`)
copies each edge's tail to the new type. When one type is content-scoped and the
other identity-scoped, the renamed edges keep tails the new type does not
expect. `ShapeProjection` records no relation scope, so the step cannot see the
change. The gap is documented on `renameRelationTypeStep`.

## Approach

- Add relation scope to `ShapeProjection`. This is a projection-format
change, so it needs the migration-file compatibility story from
`docs/data-migration.md`.
- `Validate(from, to)` refuses a rename whose scope differs, or the step gains
an explicit tail mapping.

## Acceptance

- A rename between a content-scoped and an identity-scoped type is refused at
validate time with a message naming both scopes.
- Existing migration files still load.
