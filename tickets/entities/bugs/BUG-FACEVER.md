---
id: BUG-FACEVER
type: bug
title: Entity delete versions only the default face; the other N-1 faces lose their delete marker
description: Manager.DeleteEntity reads GetEntity(id) — the default face — and records one VersionOpDelete, while the store hard-deletes the whole state family. Every non-default face's history therefore ends with no delete marker and no restore path. This is the entity half of the bug RR-181AFY fixed for cascade-deleted relations. cascadeHost.DeleteEntity captures no version at all.
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

## Fix sketch

Enumerate the family with `store.EntityQuery{IDs: []string{id}, AllStates: true}`
before the delete and capture one `VersionOpDelete` per face, the way the
relation loop directly below already does per relation. Same for rename.

Discovered while implementing TKT-C1XUA8 PR-D (`after: discard`); deliberately
left out of that PR because it touches the ordinary delete path, which has
nothing to do with copies.
