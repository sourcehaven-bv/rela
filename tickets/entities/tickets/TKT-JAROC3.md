---
id: TKT-JAROC3
type: ticket
title: Faced relation restore forks the lineage instead of replacing the face's links
kind: enhancement
priority: high
effort: s
status: ready
---

## Problem

Restoring a faced (`scope: content`) relation creates a second edge instead of
restoring the one the user asked for.

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
mints a *second, default-tailed* edge beside the faced one. Same shape in
`internal/cli/relation_history.go:199`.

The result is two edges where the user expected one, and the faced edge they
meant to restore is untouched.

## Restore is an ENTITY operation

A user does not restore one relation. They restore an entity, and its links come
along — the same way its body does. So the semantics are:

**Entity restore replaces the face's links.** Restoring a face brings back the
content-scoped edges that face had at that version, and clears whatever
content-scoped edges the face holds now. Identity-scoped edges belong to the
entity, not the face, and are untouched.

This mirrors the body exactly: restoring a version does not merge the old body
into the new one, it replaces it. A link set that merged instead would leave the
face holding edges from two different points in time, which is not a state the
entity was ever in.

## Why it matters now

Before BUG-64MU2Q no client could write a state-tailed edge, so a faced edge
could not reach restore at all. That fix added `Manager.DeleteRelationState` and
made the reconciler face-aware, so it can now.

## Fix

Thread the face through the liveness probe and into `RelationOptions.FromFace` —
`entity.RelationOptions` gained that field in BUG-64MU2Q, so the write side is
ready and only the handler needs wiring.

Then make entity restore drive the link set: for the face being restored, delete
its current content-scoped edges and write back the ones the version carried.
`Manager.DeleteRelationState` is the per-edge primitive that exists for this.

## Done: per-tail version capture

The capture half of this ticket is fixed (commit `bddf9e62`). It was smaller
than originally written: the synchronous hook already HAD the face on
`entity.Relation.FromFace`, so there was no design question about exposing
`rel_record_id` on a domain type. The fix was to stop dropping it.

- `entitymanager/version_hook.go` — removed the `if !r.FromFace.IsDefault()`
skip; `RelationVersionRecord` carries `FromFace`, exactly as an entity's
`VersionRecord` carries `Face`.
- `pgstore` / `sqlitestore` `recordIDForKey` — the key now includes the tail,
so a state-tailed capture resolves its OWN lineage. Passing the zero face asks
about the default tail, which is what a caller that never names a face means.
- `sqlitestore.WriteRelationVersion` — gained the key-resolution step pgstore
already had. Without it a synchronous capture inserted `rel_record_id = 0`,
filing every sync capture on one shared lineage. Pre-existing, found while
fixing the above.
- `pgstore.contentHashOfRelation` — now folds in `FromFace`, matching
sqlitestore and `canonical.HashRelation`. Without it two tails holding identical
bytes hash the same, and a content-keyed dedup drops one tail's capture.
- `storetest` `RelationTails` — three conformance cases holding both backends
to this: independent lineages, distinct hashes on identical bytes, and
`ErrNotFound` for a tail with neither a live edge nor history.

Reading history BY face is the remaining read-side gap:
`store.RelationHistoryQuery` has no face field, so a caller after a state-tailed
edge's history must pass its `RecordID`. Both read call sites pass the zero face
with a comment naming this.

## Related

- **BUG-64MU2Q** — added the faced write path that makes this reachable, and
the `FromFace` field the fix needs.
- **TKT-0VJ0HV** — the same default-tail assumption on the single-relation
GET/PATCH/DELETE routes.
