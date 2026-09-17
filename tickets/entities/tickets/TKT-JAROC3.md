---
id: TKT-JAROC3
type: ticket
title: 'Faced relation history: capture skips state-tailed edges, and restore forks the lineage'
kind: enhancement
priority: high
effort: s
status: backlog
---

## Problem

Two halves, both on faced (`scope: content`) relations.

### 1. Capture skips state-tailed edges

`internal/entitymanager/version_hook.go` returns early for any relation whose
`FromFace` is non-default:

```go
if !r.FromFace.IsDefault() {
    return
}
```

The reason is sound. The synchronous path builds a record carrying the `(from,
type, to)` triple, and the store resolves that through `recordIDForKey`, which
is hardcoded to `from_face = ''` (`pgstore/relation_version.go:195`,
`sqlitestore/relation_version.go:215`). Capturing a state-tailed edge there
would file it under the default tail's lineage, interleaving two edges'
histories.

Create and update are unaffected — the **sweep** captures those, reading
`rel_record_id` straight off the row. The gap is the **synchronous** path:
pre-delete capture and the rename stitch.

### 2. Restore forks the lineage (the sharper half)

`internal/dataentry/relation_history_handler.go:365`:

```go
_, liveErr := a.store.GetRelation(ctx, from, relType, to)
if liveErr == nil {
    _, writeErr = a.entityManager.UpdateRelation(ctx, from, relType, to, opts)
} else {
    _, writeErr = a.entityManager.CreateRelation(ctx, from, relType, to, opts)
}
```

`GetRelation` reads the DEFAULT tail, so for a faced edge it misses and the
handler takes the **create** branch. `opts` carries no face, so the restore
mints a *second, default-tailed* edge beside the faced one instead of restoring
it. Same shape in `internal/cli/relation_history.go:199`.

That is worse than the capture gap: capture fails safe (no history), restore
actively forks a lineage and leaves two edges where the user expected one.

## Why it matters now

Before BUG-64MU2Q no client could write a state-tailed edge, so both were
unreachable outside the copy kernel. That fix added
`Manager.DeleteRelationState` and made the reconciler face-aware, so a faced
relation delete now takes the sync path and a faced edge can reach restore.

## Fix

`store.RelationVersionInput` already carries the handle the capture comment asks
for:

```go
type RelationVersionInput struct {
    RecordID int64
    FromFace entity.Face
    ...
}
```

What is missing is a way for the entitymanager to learn the live row's
`rel_record_id`. Options:

1. Expose it on `entity.Relation` (widens a domain type for a storage
surrogate — probably wrong).
2. Give the recorder a face-aware lookup so the store resolves the lineage
from `(from, face, type, to)` — parameterizing the `from_face = ''` in
`recordIDForKey` and its `dead` fallback query.

Option 2 keeps the surrogate inside the store, which is where it belongs.

For restore: thread the face through the liveness probe and into
`RelationOptions.FromFace` — `entity.RelationOptions` gained that field in
BUG-64MU2Q, so the write side is ready and only the handler needs wiring.

## Related

- **BUG-64MU2Q** — added the faced write path that makes both reachable, and
the `FromFace` field the fix needs.
- **TKT-0VJ0HV** — the same default-tail assumption on the single-relation
GET/PATCH/DELETE routes.
