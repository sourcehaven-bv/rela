---
id: TKT-K2VAA
type: ticket
title: Extend PATCH /entities/{id} to accept relations (JSON:API-shaped)
kind: enhancement
priority: medium
effort: m
status: done
---

## Summary

Extend the entity PATCH endpoint to accept a `relations` field that fully
replaces the relation set per relation type, with optional per-edge metadata.
Borrows the JSON:API resource-identifier shape (arrays of `{id, meta?}` objects)
for uniformity.

```http
PATCH /api/v1/tickets/TKT-001
{
  "properties": { "title": "..." },
  "relations": {
    "belongs-to": [{ "id": "CAT-001" }],
    "tagged": [
      { "id": "LBL-001", "meta": { "added_by": "jeroen" } },
      { "id": "LBL-002" }
    ]
  }
}
```

## Why this ticket

Carved off from TKT-18JS6 (form auto-save). The auto-save composable currently
only handles property/content edits. Picker-style relations (`RelationPicker`)
and card-style relations (`RelationCards`) both go through separate per-edge
endpoints (`POST/PATCH/DELETE /relations/{name}/{target}`), so they cannot
participate in the single FIFO-queue auto-save flow.

This means today most edit forms have a real silent-data-loss hole: relation
edits without an explicit Save click never persist. The current workaround in
TKT-18JS6 is to keep the Save button on any form that has card relations — but
`belongs-to` etc. (no cards) STILL leak edits.

Folding relation updates into the entity PATCH:

- Single endpoint, single FIFO queue, single error path on the client.
- Atomic per-PATCH: validate desired state, apply or reject as a unit.
- Same shape covers picker (no meta) and cards (with meta) — no
schema bifurcation.
- Unblocks the daily-notes UX (TKT-18JS6 / FEAT-KTP7S): `focuses-on`
becomes another auto-saved field.

## Wire format (JSON:API-flavored)

- `relations: Record<string, RelationRef[]>` where
`RelationRef = { id: string; meta?: Record<string, unknown> }`.
- Field absence ≠ empty array. Absence means "don't touch this
relation type"; empty array means "remove all relations of this type".
- `id` only — `type` is derivable from prefix in rela (TKT-, LBL-,
...). We don't need JSON:API's full `{type, id}` strictness; one field is
enough.
- Same shape regardless of whether the relation has properties.
`meta` is optional everywhere.

## Server-side semantics

1. Compute the diff between the current relations and the desired
set per relation type appearing in `relations`.
2. Validate the *full* intended state (cardinality, allowed targets,
relation properties) before applying any changes.
3. Apply additions, deletions, and meta updates as a single
transaction. On any validation failure, return 422 with no partial state visible
to other clients.
4. Emit a single `entity:updated` SSE event after the commit.
Granular `relation:created` / `relation:deleted` events stay for compat but are
now redundant for clients that listen to entity-level updates.

## Out of scope

- Frontend wiring of `RelationPicker` and `RelationCards` to the new
endpoint (depends on this ticket; finished in TKT-18JS6 and TKT-B9SXH
respectively).
- Migration of the legacy per-relation endpoints. They stay; the new
PATCH path is purely additive.
- Full JSON:API `{type, id}` strictness — defer until/unless we adopt
the full spec.

## Depends on / unblocks

- Unblocks: TKT-18JS6 (form auto-save) and TKT-B9SXH (RelationCards
auto-save) — both can switch to the unified PATCH path once this lands.
