---
id: TKT-JAROC3
type: ticket
title: Entity restore should replace the face's content-scoped links
kind: enhancement
priority: high
effort: s
status: ready
---

## Problem

Restoring a faced (`scope: content`) relation replaces only that one edge. A
user does not restore one relation; they restore an entity, and its links should
come along.

## Restore is an ENTITY operation

**Entity restore replaces the face's links.** Restoring a face brings back the
content-scoped edges that face had at that version, and clears whatever
content-scoped edges the face holds now. Identity-scoped edges belong to the
entity, not the face, and are untouched.

This mirrors the body exactly: restoring a version does not merge the old body
into the new one, it replaces it. A link set that merged instead would leave the
face holding edges from two different points in time, which is not a state the
entity was ever in.

## Fix

Make entity restore drive the link set: for the face being restored, delete its
current content-scoped edges and write back the ones the version carried.
`Manager.DeleteRelationState` is the per-edge primitive that exists for this.

The per-relation restore route is already face-correct (see below); what is
missing is the entity-level operation that drives it.

## Done: per-tail capture and per-tail history reads

Both halves of the original ticket are fixed (commits `bddf9e62`, `5a0800cf`).
The work was smaller than first written: the face was already in hand at every
boundary, so the fix was to stop dropping it.

### Capture (`bddf9e62`)

- `entitymanager/version_hook.go` — removed the `if !r.FromFace.IsDefault()`
skip; `RelationVersionRecord` carries `FromFace`, exactly as an entity's
`VersionRecord` carries `Face`.
- `pgstore` / `sqlitestore` `recordIDForKey` — the key now includes the tail,
so a state-tailed capture resolves its OWN lineage.
- `sqlitestore.WriteRelationVersion` — gained the key-resolution step pgstore
already had. Without it a synchronous capture inserted `rel_record_id = 0`,
filing every sync capture on one shared lineage. Pre-existing, found while
fixing the above.
- `pgstore.contentHashOfRelation` — now folds in `FromFace`, matching
sqlitestore and `canonical.HashRelation`. Without it two tails holding identical
bytes hash the same, and a content-keyed dedup drops one tail's capture.

### Reads (`5a0800cf`)

- `store.RelationHistoryQuery` gained `FromFace`, and
`RelationHistoryReader.ListRelationLifetimes` gained a `fromFace` parameter.
Enumerating lifetimes across all tails would hand a caller asking about the
draft edge an opaque `RecordID` belonging to the published edge, and the
response carries no face to tell them apart.
- **`internal/dataentry/relation_history_handler.go` never parsed the
`{from}` segment.** On a faced type `selfHref` hands out `ID@face` and the SPA
passes it verbatim, so two things failed silently: the ACL row gate got a
suffixed string that matches no row (a 404 for an ordinary reader on pgstore),
and the store read the default tail's history. It now parses with
`parseEntityRef`, like every other faced route.
- The per-relation restore route writes back to the tail it read from
(`RelationOptions.FromFace`) and probes liveness on that tail via a new shared
`edgeOnFace`. Previously a faced restore took the create branch and minted a
second, default-tailed edge beside the one the caller addressed.
- `internal/cli/relation_history.go` accepts `ID@face` on the `from`
positional for both `relation-history` and `relation-restore`.
- Version purge stays default-tail: `RelationVersionPurgeRequest` names no
face, and each call site says so.

### Tests

- `storetest` `RelationTails` holds both database backends to one contract:
independent lineages per tail, reads scoped by face returning that tail's
snapshot, distinct content hashes on identical bytes, and `ErrNotFound` for a
tail with neither a live edge nor history.
- `internal/dataentry` pins both directions of the address parse — a faced
address reads its own tail, a bare one reads the default tail. The test fake
keys on the tail exactly as the real store does, so a handler that drops the
face fails it.

## Related

- **BUG-64MU2Q** — added the faced write path that makes this reachable, and
the `FromFace` field the fix needs.
- **TKT-0VJ0HV** — the same default-tail assumption on the single-relation
GET/PATCH/DELETE routes.
