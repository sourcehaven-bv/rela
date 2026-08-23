---
id: BUG-FACEVER
type: bug
title: "Versioning is not face-aware end to end: delete captures the default face only, the sync relation path cannot address a tail, and capture is not transactional"
description: "Three facets of one defect — content versioning is per-(id,pointer) in the schema but the write choke-point still addresses faces by bare id. (1) Manager.DeleteEntity records one VersionOpDelete from GetEntity(id) while the store sweeps the whole family. (2) recordRelationVersion must skip state-tailed edges because recordIDForKey has no tail predicate, so a discarded face's edges lose their delete marker. (3) Capture is best-effort outside the store.Tx, so a rollback can leave a delete version for a live row. Fixing them separately would mean touching the same seam three times."
priority: medium
status: backlog
---

## Symptom

`internal/entitymanager/manager.go:909-943`:

```go
current, err := m.deps.Store.GetEntity(ctx, id)   // :910 — DEFAULT FACE ONLY
...
m.recordEntityVersion(ctx, store.VersionOpDelete, current, "")  // :943
```

`GetEntity(id)` addresses the bare id, which is the default state. The store's
`DeleteEntity` then sweeps the **whole state family** — pgstore
`entity.go:467-469` says so explicitly ("the `WHERE id = $1` statements below
sweep every state row"), and `storetest/states.go` pins it as
`DeleteCascadesTheFamily`.

So deleting an N-face entity hard-deletes N rows and records **one** delete
version.

## Impact

The version schema is fully `(entity_id, pointer)`-keyed — `insertVersion`
writes the pointer as column 2 and folds it into the content hash
(`pgstore/version.go:44-88`), and lineage reads filter
`AND pointer = CAST($2 AS text)`. Per-state versioning landed in TKT-C1XUA8
PR-B. But nothing on the delete path enumerates the family to feed it:
`AllStates` appears **nowhere** in `internal/entitymanager`.

Each non-default face's `entity_versions` lineage therefore ends on whatever
`create`/`update` the sweep last captured, with no `delete` row. There is no
restore path and no record that the face ever went away.

This is precisely the failure `RR-181AFY` fixed for cascade-deleted
**relations** — see the comment at `manager.go:952-955`: *"This is the ONLY
place cascade-deleted relations get versioned: the store's DeleteEntity
bulk-deletes them below the write choke-point, so without this their history
would silently end with no delete marker and no restore path."* The entity
half of the same defect was never closed.

`Manager.RenameEntity` has the same shape at `manager.go:1068`: one rename row
for the default face while the store re-keys every face
(`storetest/states.go` `RenameCascadesTheFamily`).

`cascadeHost.DeleteEntity` (`internal/entitymanager/cascadehost.go:172-208`)
is worse — it calls the store delete at :195 with **no version capture at
all**, only a `recordCascade` audit at :207.

## Facet 2 — the sync relation path cannot address a tail

`recordRelationVersion` (`internal/entitymanager/version_hook.go`) deliberately
**returns early for any edge whose `FromPointer` is non-default**, and the
reason is sound: the synchronous record carries only the `(from, type, to)`
triple, and pgstore resolves that to a lineage via `recordIDForKey`
(`internal/store/pgstore/relation_version.go`), whose live lookup is

```sql
SELECT rel_record_id FROM relations
WHERE from_id = $1 AND rel_type = $2 AND to_id = $3 AND from_pointer = ''
```

— **no tail predicate on the caller's side**. Pushing a state-tailed edge
through it would file that edge's delete under a **sibling face's** lineage.

That is worse than the gap it would close: a missing delete marker is
recoverable, a delete marker on the wrong face's history is corruption of a
face nothing touched, and it would be silent.

Consequence today: when `after: discard` (TKT-C1XUA8 PR-E) consumes a face,
that face's outgoing edges are hard-deleted with **no delete version**. They
ARE audited (`OpDeleteRelation`, `triggered_by: copy:<name>`), so the loss is
visible and attributable — but the history ends without a marker.

**Do not "fix" this by removing the skip.** The skip is load-bearing; the
actual fix is facet 4 below.

## Facet 3 — capture is not transactional with the write

`VersionRecorder` is a Manager-level dependency writing on its own connection,
and capture is best-effort by contract (errors logged, never propagated). So:

- capture succeeds, transaction rolls back → a delete version for a live row
  (recoverable noise);
- capture fails, transaction commits → the row is gone with no marker.

`Manager.DeleteEntity` already admits this in a comment: *"the store delete is
still not transactional with this capture; strict atomicity is a future
hardening."* Same window, now on a second path.

## Fix sketch (one seam, not three)

1. Enumerate the family with `store.EntityQuery{IDs: []string{id}, AllStates: true}`
   before delete/rename and capture one version per face — the way the relation
   loop directly below already does per relation.
2. Give the synchronous relation record a **tail** (or a `rel_record_id`) so
   `recordIDForKey` can address the right lineage, then drop the non-default
   skip. This is what unblocks facet 2.
3. Capture inside the `store.Tx` — needs a view-scoped recorder. **Care
   required**: the pgstore sweep holds `pg_try_advisory_lock` per tick and
   issues its inserts on the held connection; a second writer inside a copy's
   transaction interacts with that guarantee, so this is not a rider on a
   feature PR.

All three touch the same boundary between the write choke-point and the
version store, which is why they are one ticket.

Discovered while implementing TKT-C1XUA8 PR-D/PR-E; deliberately left out of
both because they touch the ordinary delete path and the versioning seam,
neither of which is what those PRs are about.
