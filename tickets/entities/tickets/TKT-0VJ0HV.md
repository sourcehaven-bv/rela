---
id: TKT-0VJ0HV
type: ticket
title: 'Single-relation routes are default-tail only: a faced edge is 404 on GET and undeletable by triple'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

BUG-64MU2Q made content-scoped relations writable on a faced source through the
**bulk reconciler** (`PATCH /{plural}/{id}@{face}` with a `relations:` body).
The **single-relation** routes were not part of that change and remain
default-tail only:

| Route | Handler | Lands on |
|---|---|---|
| `GET /{plural}/{id}/relations/{type}/{target}` | `relation_read_handler.go:47` | `store.GetRelation` — default tail |
| `PATCH` same route | `write_handler.go` | `manager.UpdateRelation` — default tail |
| `DELETE` same route | `write_handler.go` | `manager.DeleteRelation` — default tail |

None call `parseEntityRef` on the path segment, so `ID@face` is not even parsed
there.

Consequence: an edge created via the reconciler on `POL-1@published` is
invisible to the single-relation GET and undeletable via the single-relation
DELETE — both 404. The 404 tells the client the edge does not exist, which is
now false. The bulk reconciler is the only door.

## Not harmful, just incomplete

No wrong-row write: `DeleteRelation`/`UpdateRelation` address an exact (default)
tail, so a faced edge is missed rather than confused with another. The SPA's
relations panel uses the reconciler, not these routes, so the reported bug is
fixed without them.

## Fix

Parse the path segment with `parseEntityRef` and thread `ref.Face` into
`GetRelation`/`UpdateRelation`/`DeleteRelation` — the faced store methods
(`UpdateRelationState`, `DeleteRelationState`) and
`entitymanager.DeleteRelationState` already exist, so this is wiring plus a
faced read.

A faced read needs either `store.GetRelationState` (not yet on the interface;
sqlitestore already has a private `getRelationState`) or the
`RelationQuery.FromFace` pattern that `entitymanager.getRelationOnFace` uses.

## Related

- **BUG-64MU2Q** — the reconciler half, done.
- **TKT-JAROC3** — relation history/restore on a faced path, which has the
sharper version of this problem: `relation_history_handler.go:365` does
`GetRelation` → falls to the create branch → mints a *second*, default-tailed
edge instead of restoring the faced one. That one forks a lineage rather than
404ing, so it is the more urgent of the two.
