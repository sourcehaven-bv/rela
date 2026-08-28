---
id: RR-GM92KZ
type: review-response
title: 'Double GetEntity: pass oldEntity into updateCore instead of re-reading'
finding: 'As planned, PatchEntity does its own GetEntity and then updateCore does a SECOND GetEntity for oldEntity (manager.go:573) — two reads of the same row per patch. Not a new race (the existing lua pattern runtime.go:1740 GetEntity -> :1747 Clone -> :1766 UpdateEntity has the same read-modify-write gap) and not corruption: the second read only feeds oldEntity for automation and EnforceUpdate, so a concurrent write between the two reads makes automation diff against a NEWER old-state than the patch was computed from. Impossible on fsstore/memstore (in-process single-writer by design); reachable on pgstore multi-writer. Wrapping the pipeline in store.Tx is NOT the fix — the cascade runs arbitrary Lua, and CLAUDE.md/store.go:204-211 forbid slow external I/O inside a Tx callback. The clean fix is to have updateCore accept the already-fetched oldEntity as a parameter rather than re-reading it. UpdateEntity fetches it at manager.go:573 and PatchEntity fetches it too, so threading it through makes the extraction REDUCE reads rather than double them, and narrows the window without a transaction.'
severity: minor
resolution: |-
    ACCEPTED. updateCore takes the already-fetched oldEntity as a parameter instead of re-reading it. Both entry points already hold it at the call point: UpdateEntity reads it at manager.go:573, and PatchEntity must read it anyway to learn the entity type for the ACL subject (RR-0V0TVB) and to compute the merge base. Net effect: one read per write on both paths, down from two on the patch path — the extraction reduces reads rather than doubling them.

    Explicitly NOT doing: wrapping in store.Tx. All three backends implement it (fsstore/tx.go:31, memstore/tx.go:22, pgstore/tx.go:60), but the cascade dispatches arbitrary Lua and both root CLAUDE.md and store.go:204-211 forbid slow external I/O inside a Tx callback. Optimistic concurrency stays out of scope per the ticket; PatchEntity is documented as last-write-wins per property, which is still strictly better than today's whole-entity last-write-wins.
status: addressed
---

## Resolution

Signature becomes roughly:

```go
func updateCore(ctx context.Context, deps Deps, e, oldEntity *entity.Entity, ...) (*entity.UpdateResult, error)
```

Both entry points already hold `oldEntity` at the point they would call it:

- `UpdateEntity` — reads it at `manager.go:573`
- `PatchEntity` — must read it anyway to learn the type for the ACL subject
(RR-0V0TVB) and to compute the merge base

Net effect: one read per write on both paths, down from two on the patch path.

Explicitly **not** doing: wrapping in `store.Tx`. All three backends implement
it (`fsstore/tx.go:31`, `memstore/tx.go:22`, `pgstore/tx.go:60`), but the
cascade dispatches arbitrary Lua, and root `CLAUDE.md` plus
`internal/store/store.go:204-211` forbid slow external I/O inside a `Tx`
callback. Optimistic concurrency remains out of scope per the ticket.
