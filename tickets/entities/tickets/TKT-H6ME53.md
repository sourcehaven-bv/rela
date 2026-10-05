---
id: TKT-H6ME53
type: ticket
title: 'Soft delete per face: Undo for face deletes'
kind: enhancement
priority: medium
effort: m
status: backlog
description: Face deletes are permanent; only a whole-entity delete is soft. Make soft delete work per face so Undo covers faced types.
---

## Description

Soft delete (`store.SoftDeleter`) marks a whole entity family by id. A face
delete (`DeleteEntityFace`) is permanent. Since a data-entry DELETE of a faced
type must now name a face (bare id is `face_required`), faced types have no
Undo: the bulk-delete Undo toast and restore fail for them.

At the storage layer a face is just a row, so soft delete should apply per face,
the same as history does.

## Scope

- `store.SoftDeleter`: mark, unmark, list and purge keyed by `entity.Ref`, in fsstore, memstore, pgstore (migration) and sqlitestore; `storetest` conformance.
- Which relations a face mark hides: its own content-scoped edges; the last face hides every incident edge, as `DeleteEntityFace` does.
- Create/rename conflict rules for a marked face.
- `entitymanager` soft delete and restore per face; restore handler; purge job.
- SPA: bulk-delete Undo and the entity-page delete for faced rows.

Overlaps the deferred restore redesign (BUG-KK1UXH).
