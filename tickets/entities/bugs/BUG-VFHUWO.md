---
id: BUG-VFHUWO
type: bug
title: 'On postgres, a faced entity''s relations, clone and document routes are unreachable: the id segment is never parsed'
description: Ten routes take the {id} path segment verbatim and pass it to getEntity/store.GetEntity, which is by-contract the bare id. On a type that DECLARES faces there is no zero-coordinate row, so `_self` (ID@face) is the only address a client has — and on pgstore that address resolves to nothing. The whole /{plural}/{id}/relations sub-tree (GET, POST, PATCH, DELETE), _actions/clone, and the entity-anchored document + export routes are affected. Invisible in tests because internal/dataentry tests run on memstore, which keys its map on FormatStateRef and so resolves the suffixed string by accident.
priority: high
effort: m
why1: The /{plural}/{id}/relations sub-tree, _actions/clone and the entity-anchored document routes passed the raw {id} path segment to store.GetEntity, which is the BARE id by contract. On a faced type the SPA sends ID@face (selfHref's output), so the lookup matched no row and every one of those routes 404'd.
why2: 'The bare id was no escape either: a type declaring faces stores nothing at the zero coordinate (BUG-HC6I2T), so both addresses missed and no working address existed.'
why3: These routes were written before faced types existed, when {id} was only ever a bare id. TKT-O7R2A1 later added the face half of the READ GATE to them, so they became face-aware for authorization while staying face-blind for addressing — and the partial fix made them look finished.
why4: No test could see it. internal/dataentry fixtures are memstore-only, and memstore keys its index on FormatStateRef, so a suffixed id resolves by accident and a status-code assertion passes against the broken code. The defect reproduces only on pgstore, which looks up by (id, face) columns.
why5: A bare id and an ID@face address are both `string` at the route boundary, so the compiler cannot distinguish a parsed address from an unparsed one — and the backend that would expose the difference is not the one the tests run against. Third occurrence of this shape (BUG-64MU2Q, BUG-OOZBBK, this one).
prevention: 'Two layers. (1) A test for this class must assert WHICH ROW was reached — which edges came back, which edge was written — never a status code, because memstore''s accidental resolution makes a 200-assertion pass against broken code. That is not hypothetical: my first attempt at the multi-tail test passed against both implementations because memstore yields tails in sorted order, and only addressing the tail that does NOT sort first made it discriminate. (2) The structural fix is a distinct address type at the route boundary so `string` cannot stand in for a parsed address; entityRef already exists and is what the correct routes use. Three occurrences is enough evidence the per-route test is not holding on its own — see measure faced-route-address-parse-test.'
status: done
---

## Symptom

On a **postgres** deployment, for an entity type that declares `faces:`, these
return 404 for every caller:

- `GET|POST /api/v1/{plural}/{id}@{face}/relations[/{relType}[/{target}]]`
- `PATCH|DELETE` on the same relation paths
- `POST /api/v1/{plural}/{id}@{face}/_actions/clone`
- `GET /api/v1/_documents/{doc}/{id}@{face}` and its `/_export`

The bare id returns 404 too, because a faced type stores nothing at the zero
coordinate (BUG-HC6I2T). So there is no address that works, and the SPA only
ever has `_self`, which is the faced one.

## Verified on a running server

Reproduced against `rela-server-postgres` on a real PostgreSQL database, using
`prototypes/worlds/project` (whose `policy` type declares `draft`/`published`
and whose `implements` relation is `scope: content`).

**ACL was removed from the project for the measurement**, so every result below
is pure addressing with no grant in play:

```
entity GET  faced (POL-002@draft)            -> 200   PARSED route works
relations   faced (POL-002@draft/relations)  -> 404   RAW route fails
entity GET  bare  (POL-002)                  -> 404   no default-face row
relations   bare  (POL-002/relations)        -> 404

POST /policys/POL-002@draft/relations/implements  -> 404
POST /policys/POL-002@draft/_actions/clone        -> 404
```

The entity GET returning 200 on the same address the relations route 404s is the
whole finding: one route parses, the other does not.

**Worlds are not the escape hatch.** The relations route explicitly REFUSES
`?world=`:

```
GET /policys/POL-002@draft/relations?world=editorial
  -> 422 world_unsupported
     "this endpoint serves the default world only; omit ?world="
```

So there is no world under which the faced edge becomes reachable on this route.

## Cause

Ten handlers take the `{id}` path segment verbatim and pass it to
`entityReader.getEntity`, which is `store.GetEntity` — by contract the BARE id
addressing the default state.

```
memstore.GetEntity(id) → m.entities[FormatStateRef(id, "")]
```

`FormatStateRef("POL-002@draft", "")` is the literal `"POL-002@draft"` — the key
the faced row is stored under — so memstore resolves it **by accident**. pgstore
does `WHERE id = $1 AND face = $2`, so the same call looks for an entity whose
id COLUMN is `POL-002@draft` and misses.

## Affected routes

| Route | Handler | Extraction |
|---|---|---|
| `GET /{plural}/{id}/relations` | `handleV1EntityRelations` (`api_v1.go:1122`) | `parts[1]` raw |
| `GET /{plural}/{id}/relations/{relType}` | `handleV1GetRelationType` (`api_v1.go:1340`) | raw |
| `POST /{plural}/{id}/relations/{relType}` | `handleV1CreateRelation` (`write_handler.go:970`) | raw |
| `GET /{plural}/{id}/relations/{relType}/{target}` | `handleV1GetRelationTarget` (`relation_read_handler.go:30`) | raw |
| `PATCH .../relations/{relType}/{target}` | `handleV1UpdateRelation` (`write_handler.go:1066`) | raw |
| `DELETE .../relations/{relType}/{target}` | `handleV1DeleteRelation` (`write_handler.go:1149`) | raw |
| `POST /{plural}/{id}/_actions/clone` | `handleV1CloneEntity` (`write_handler.go:1191`) | raw |
| `GET /_documents/{doc}/{id}` | `resolveAnchoredDocument` (`export_document.go:120`) | `isSafePathSegment` only, then `a.store.GetEntity` DIRECTLY |
| `GET /_documents/{doc}/{id}/_export` | shares the above | raw |

The dispatcher hands `parts[1]` through unparsed at `api_v1.go:227, 238, 240,
255` — the `_attachments` branches beside them DO call `bareEntityID`, with a
comment explaining why bare is right there. The relations and `_actions`
branches simply lack it.

## What is NOT affected

The entity GET/PATCH/DELETE, history, relation-history, views, side-panel and
export routes all call `parseEntityRef`. Attachments call `bareEntityID`
deliberately (files are keyed by bare entity id in every backend) — correct, not
a defect.

**This is not an authorization hole.** TKT-O7R2A1 already added the face half of
the read gate to the relations route via `faceReadable`, pinned by
`facegate_surfaces_test.go`. These routes fail CLOSED: a caller sees 404 where
they should see their data. Reachability, not disclosure.

## Why the tests do not catch it

`internal/dataentry`'s fixtures are memstore-only, and memstore's index key IS
the state reference — so the suffixed string resolves and every test passes. The
same asymmetry hid BUG-OOZBBK.

A test for this must either assert on the EXTRACTION (that the handler produced
a bare id and a face separately) or run against pgstore. A memstore test
asserting "faced address returns 200" passes against the broken code.

## Fix

Call `parseEntityRef` on the `{id}` segment and read through the face, as the
already-correct routes do. `entityReader.getEntityRef` (`entityreader.go:61`) is
the existing face-aware read; the raw sites call `getEntity` instead.

`resolveAnchoredDocument` additionally bypasses `entityReader` and calls
`a.store.GetEntity` directly, and the segment keys the on-disk render cache, so
a faced address would key a distinct entry. Handle both together.

Open question for planning: the relations route currently refuses `?world=`
(422). Parsing the address does not change that, but it is worth deciding
whether a faced address and a world-scoped read should ever be offered on the
same route, or whether the address stays the only spelling here.

## Related

- **BUG-OOZBBK** — the same defect on the relation-history route; this bug is
what its prevention measure's "audit the other routes" item found.
- **BUG-64MU2Q** — the same shape on the relation write path.
- **TKT-O7R2A1** — added the face half of the READ GATE to these routes; it
did not address addressability.
