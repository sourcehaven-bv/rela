---
id: TKT-37TYZ2
type: ticket
title: 'Sync read paths bypass read ACL (IB-review #1)'
kind: enhancement
priority: high
effort: s
status: done
---

Addresses the blocking IB-review (CISO) finding #1 on rela#1010 / TKT-GFJJ3S.

## Problem

The sync API's read paths read straight from `store` / `ManifestSince` with no
read-ACL check, while every other read path in the app is gated:

- `handleSyncGet` (`GET /api/sync/<kind>/<id>`) called `store.GetEntity` /
  `store.GetRelation` and returned the record unfiltered.
- `handleSyncManifest` (`GET /api/sync/manifest`) returned the full
  entity/relation/tombstone change set unfiltered.

Any authenticated principal could thus read all entities/relations — and learn
the full id/relation set via the manifest — regardless of their role's read
rights. (Write paths were already correctly gated via
`entitymanager.authorizeAndAudit`.)

## Fix

Apply the same read gate the rest of data-entry uses (`readGate.PermitsRead` /
`PermitsReadMany`, `readGateFromContext`):

- Entity reads gate on `(type, id)`.
- Relation reads gate on the **source (From) entity**, mirroring
  `handleV1EntityRelations` (a relation has no type of its own; entity
  tombstones carry `typ`, relation tombstones gate on the From id).
- `handleSyncManifest` filters every row to the principal's read scope
  (batched `PermitsReadMany` per type) before serializing.

Denied reads 404 indistinguishably from not-found (no body/ETag leak). The
manifest cursor still advances to the highest seq over **all** rows (visible or
not) so a client never re-polls the hidden tail forever.

## Tests

`TestSync_GetEntity_ACLDenied`, `TestSync_GetRelation_ACLDenied`,
`TestSync_Manifest_ACLFiltered` — deny path 404s, no leak, cursor advances past
hidden rows. Existing NopACL sync tests stay green (no behavior change without a
policy).
