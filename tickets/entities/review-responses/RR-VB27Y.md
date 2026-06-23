---
id: RR-VB27Y
type: review-response
title: 'Exact store types to map: errors, query/result structs, entity timestamps'
finding: 'Plan was vague on exact contract types. Concrete facts to implement against: sentinel errors are ErrNotFound, ErrConflict, ErrHasRelations (store.go:21-26) — note ErrHasRelations exists (non-cascade delete of an entity that still has relations). EntityQuery{Type, IDs[], Cursor, Limit}; RelationQuery{From, To, Type, EntityID, Direction(Both/Outgoing/Incoming), Cursor, Limit}. Page[T]{Items, NextCursor}. DeleteResult{DeletedEntities[], DeletedRelations[]}. RenameResult{RelationsUpdated int}. RelationData{Properties, Content}. AttachmentInfo{EntityID, Property, FileName, ContentType, Size}. entity.Entity has an UpdatedAt time.Time field already — the store sets/returns it (so map DB updated_at -> Entity.UpdatedAt). HighestID parses numeric suffix after ''PREFIX-'' via Sscanf %d, skips non-matching/non-numeric, returns max (gaps ignored), 0 if none.'
severity: minor
resolution: 'Implemented (commit 296c5f3f): pgstore maps the exact contract types — ErrNotFound/ErrConflict/ErrHasRelations, EntityQuery/RelationQuery filters, Page[T] keyset cursors, DeleteResult/RenameResult, RelationData, AttachmentInfo, and DB updated_at -> entity.UpdatedAt. The full storetest.RunAll conformance suite (which asserts all of these) passes with -race.'
status: addressed
---

## Resolution (plan update)

Add a "contract types" appendix to the plan and implement precisely:
- Map DB `updated_at` -> `entity.Entity.UpdatedAt` and `entity.Relation.UpdatedAt` on read (the FS/mem stores populate these; conformance may assert monotonic update times).
- `DeleteEntity(cascade=false)` on an entity with relations returns `ErrHasRelations` (not a generic error); `cascade=true` returns `DeleteResult` listing all deleted entities+relations (drives the cascade watcher events).
- `RenameEntity` runs in one transaction, rewrites the entity PK and every relation endpoint, returns `RenameResult{RelationsUpdated}`.
- Pagination: implement keyset/offset cursor producing `Page[T]{Items, NextCursor}`; `Limit==0` means no limit; `ListEntities`/`ListRelations` (iterator) ignore Cursor/Limit, only the `...Page` variants honor them.
- `RelationQuery.EntityID` + `Direction` select endpoint(s): Both => from OR to; Outgoing => from; Incoming => to.
- `PropertyValues(property, limit)` => distinct values ordered by frequency desc (GROUP BY ... ORDER BY count(*) DESC LIMIT).
- Properties/RelationData.Properties stored as JSONB; round-trip nested values without mutation (FuzzCloneNestedValues).
