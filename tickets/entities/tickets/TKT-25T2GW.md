---
id: TKT-25T2GW
type: ticket
title: Entity restore replaces the face's content-scoped links
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Problem

Restoring a relation is a per-edge operation today. That is not how a user
thinks about restore: they restore an ENTITY, and its links should come along.

## Semantics

**Entity restore replaces the face's links.** Restoring a face brings back the
content-scoped edges that face had at that version, and clears whatever
content-scoped edges the face holds now. Identity-scoped edges belong to the
entity rather than the face, so they are untouched.

This mirrors the body exactly. Restoring a version does not merge the old body
into the new one, it replaces it. A link set that merged instead would leave the
face holding edges from two different points in time — a state the entity was
never actually in, so no version of it is a faithful record.

## Fix

For the face being restored, delete its current content-scoped edges and write
back the ones the version carried. `entitymanager.Manager.DeleteRelationState`
is the per-edge primitive that exists for this, and the per-relation restore
route is already face-correct (TKT-JAROC3), so what is missing is the
entity-level operation that drives them.

Open questions for planning:

- Where the restored link set comes from. Relation versions are keyed per edge,
so "the edges this face had at version N" is a query across lineages by
timestamp, not a single stored set.
- What happens when a restored edge's target no longer exists. The per-relation
route returns 409 `dangling_endpoint`; an entity restore touching many edges
needs a policy for partial failure.
- Whether the body restore and the link restore must be atomic.

## Related

- **TKT-JAROC3** — carried the source face through capture and history reads;
this was its second half, split out because that work shipped and this did not.
- **BUG-64MU2Q** — added the faced relation write path this depends on.
