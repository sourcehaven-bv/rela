---
id: TKT-JAROC3
type: ticket
title: Synchronous relation version capture skips state-tailed edges, so a faced relation delete records no history
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Problem

`internal/entitymanager/version_hook.go:152` skips version capture for any
relation whose `FromFace` is non-default:

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

## Why it matters now

Before BUG-64MU2Q no client could write a state-tailed edge, so the skip was
unreachable outside the copy kernel. That bug added
`Manager.DeleteRelationState`, so a faced relation delete from the SPA now takes
this path and silently records no history. The edge is deleted correctly; only
its final version is missing.

## Fix

`store.RelationVersionInput` already carries the handle the comment asks for:

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

Option 2 keeps the surrogate inside the store, which is where it belongs. It is
a small change in two backends plus the skip removal.

## Not a regression

The skip fails safe: no history rather than wrong history. This ticket is about
closing the gap, not repairing damage.
