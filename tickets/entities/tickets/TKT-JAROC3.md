---
id: TKT-JAROC3
type: ticket
title: 'Faced relation history: capture skipped state-tailed edges and reads addressed the default tail'
kind: enhancement
priority: high
effort: s
status: done
---

## Problem

Two halves, both on faced (`scope: content`) relations, where the SOURCE face is
part of an edge's identity.

### 1. Capture skipped state-tailed edges

`internal/entitymanager/version_hook.go` returned early for any relation whose
`FromFace` was non-default. The reason was sound at the time: the store's
key-to-lineage resolver matched only `from_face = ''`, so capturing a faced edge
would have filed it under the DEFAULT tail's lineage and interleaved two edges'
histories. The skip traded a missing history for a corrupted one.

Create and update were unaffected (the sweep reads `rel_record_id` off the row).
The gap was the SYNCHRONOUS path: pre-delete capture and the rename stitch.

### 2. Reads addressed the default tail

`store.RelationHistoryQuery` had no face, so every history read resolved the
default-tail lineage. Worse, `internal/dataentry/relation_history_handler.go`
never parsed the `{from}` path segment at all — see BUG-OOZBBK, split out of
this ticket.

## Fix

The face was already in hand at every boundary, so the fix was to stop dropping
it. There was no design question about exposing `rel_record_id` on a domain
type, which is how this was originally written up.

### Capture (`bddf9e62`)

- `entitymanager/version_hook.go` — removed the skip;
`RelationVersionRecord` carries `FromFace`, exactly as an entity's
`VersionRecord` carries `Face`.
- `pgstore` / `sqlitestore` `recordIDForKey` — the key now includes the tail,
so a state-tailed capture resolves its OWN lineage. Passing the zero face asks
about the default tail, which is what a caller that never names a face means.
- `sqlitestore.WriteRelationVersion` — gained the key-resolution step pgstore
already had. Without it a synchronous capture inserted `rel_record_id = 0`,
filing every sync capture on one shared lineage. Pre-existing and separate from
the face problem; found while fixing it.
- `pgstore.contentHashOfRelation` — now folds in `FromFace`, matching
sqlitestore and `canonical.HashRelation`. Without it two tails holding identical
bytes hash the same, and a content-keyed dedup drops one tail's capture — a
MISSING version, which is the harder bug to notice.

### Reads (`5a0800cf`)

- `store.RelationHistoryQuery` gained `FromFace`;
`RelationHistoryReader.ListRelationLifetimes` gained a `fromFace` parameter.
Enumerating lifetimes across all tails would hand a caller asking about the
draft edge an opaque `RecordID` belonging to the published edge, and the
response carries no face to tell them apart.
- The per-relation restore route writes back to the tail it read from
(`RelationOptions.FromFace`) and probes liveness on that tail via a shared
`edgeOnFace`. Previously a faced restore took the create branch and minted a
second, default-tailed edge beside the one the caller addressed.
- `internal/cli/relation_history.go` accepts `ID@face` on the `from`
positional for `relation-history` and `relation-restore`.
- Version purge stays default-tail: `RelationVersionPurgeRequest` names no
face, and each call site says so explicitly rather than by omission.

### Tests

- `storetest` `RelationTails` holds both database backends to one contract:
independent lineages per tail, reads scoped by face returning that tail's
snapshot, distinct content hashes on identical bytes, and `ErrNotFound` for a
tail with neither a live edge nor history. Verified to fail before the fix —
both captures landed on lineage 0 with two versions on it.
- `internal/dataentry` pins both directions of the address parse: a faced
address reads its own tail, a bare one reads the default tail. The test fake
keys on the tail exactly as the real store does, so a handler that drops the
face fails it.

## Related

- **BUG-64MU2Q** — added the faced write path that made both halves
reachable, and the `FromFace` field this fix needed.
- **BUG-OOZBBK** — the unparsed `{from}` address on the relation-history
route, split out of this ticket during implementation.
- **TKT-25T2GW** — entity restore replaces the face's links (the remaining
design work, moved out of this ticket).
- **TKT-0VJ0HV** — the same default-tail assumption on the single-relation
GET/PATCH/DELETE routes.
