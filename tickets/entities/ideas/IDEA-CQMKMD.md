---
id: IDEA-CQMKMD
type: idea
title: 'Snapshot-versioned ACL: transaction-id / point-in-time read verdicts for cascade deletes, _position neighbor-gap, and audit time-travel'
status: captured
---

Captured 2026-06-13 from the TKT-POT9GQ (SSE visibility) design discussion. Not
needed for POT9GQ (the cacheId scheme sidesteps it) but it's the *fully correct*
answer to a class of problems and worth recording while the reasoning is fresh.

## The core problem

A read-permission verdict is a function of graph state at a point in time. For
most reads the relevant time is "now" and the graph is live, so `PermitsRead`
resolves fine. But several problems need the verdict evaluated against a
**past** graph state, and rela has no mechanism for that today:

1. **Cascade deletes.** Deleting PRJ-42 cascades to its children (TKT-001) and tears down the conferring chain (`editor-of PRJ-42` + `belongs-to`) in the SAME operation. By the time you ask "could Alice read TKT-001?", the chain that made it readable is gone. The verdict is unrecoverable against live state. You'd need to evaluate against the graph *immediately before the cascade*.

2. **`_position` neighbor-gap analysis** (deferred from TKT-VMD8/VQGN, RR-NDMN/RR-37IY/RR-ATSO). Reasoning about which neighbors a principal could see at a given ordinal has the same "verdict at a point in time" shape.

3. **Audit time-travel.** "What could principal P read on date D?" for compliance/forensics is the same primitive.

## The mechanism (sketch — not designed)

Every write carries a monotonic transaction/snapshot id (pgstore already HAS
this: WAL LSN / the `seq` that's currently consumed in the listener and
discarded — listener.go:276; store.Event carries no seq today). Events and audit
rows record the snapshot id. A verdict query can then resolve
`PermitsRead(principal, type, id, asOf: snapshotId)` against the graph as it was
at that snapshot.

This is MVCC snapshot isolation applied to ACL evaluation. Postgres gives it
nearly for free (LSN + time-travel queries / logical decoding at an LSN);
fsstore/memstore have no MVCC and would need either a tombstone+version log or
to declare this postgres-only. PowerSync/ElectricSQL (from the SSE research)
rely on exactly this snapshot-consistency at the store layer.

## Why it's an epic, not a ticket

- `store.Event` must grow a snapshot/seq field, threaded through all three backends.
- The graph/ACL layer must support `asOf` evaluation — trivial-ish in pgstore (it has the history), a major new construct in fsstore/memstore (no point-in-time view exists).
- Likely depends on or pairs with soft-delete/tombstones (you need the deleted rows AND their relations queryable as-of a snapshot).

## Relationship to shipped work

- TKT-POT9GQ (SSE) does NOT need this — opaque cacheIds make the delete payload meaningless to non-holders, sidestepping verdict reconstruction entirely. But if POT9GQ's delete handling were ever upgraded from "opaque + type-pre-filter" to "precise per-entity verdict", THIS is the prerequisite.
- Soft-delete/trash feature ticket (separate) gives the rows survivability but NOT snapshot consistency — this idea is what makes "include deleted in an ACL query" a coherent point-in-time view rather than a race-prone live read of partially-tombstoned chains.

## Decision

Recorded as a future epic. Promote to research (`/research`) when a concrete
deployment need surfaces (correct cascade-delete propagation to scoped
principals, or compliance audit time-travel). Not scheduled.
