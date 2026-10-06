---
id: BUG-ISJHML
type: bug
title: Content-scoped edges are served on faces that do not own them
description: A draft face's content-scoped edge appears on the published face and as an incoming neighbour to a published-only reader, disclosing draft content.
priority: high
effort: m
why1: The _views traversal and several bare-id readers (export, list export, relation filter, table relation columns, write responses) query relations by bare id with no tail filter, so every face's content edges are served on every face.
why2: The world neighbour reader filtered incoming content edges by the TARGET's face, but a content edge's tail is a face of its SOURCE; so it never checked which face of the source the world serves.
why3: The FromFace tail was added to relations as a store filter (DOFYR1) and each read path had to opt in; nothing forced a read path to decide which face owns an edge.
why4: Faces landed incrementally and most read paths were written for faceless entities, where the tail is always the zero face and a bare-id query is exact.
why5: There is no single resolver that turns (entity, world) into its edges, and no fixture exercised a content edge on a non-default face until the TKT-WCMW47 e2e fixture.
prevention: Content edges are filtered through one ownership rule (ownedByFace / RelationReader.Owns) on every served surface; contentedge_face_test.go pins views, entity GET, include, list rows, relation filter, table columns and export for an all-faces and a published-only reader. The Stage 1 resolver (TKT-2528AB) should replace the per-surface checks.
status: done
---

## Problem

Observed in the TKT-WCMW47 e2e fixture (PR #1711): a content-scoped edge from
`POL-1@draft` to `CTL-1` is served on faces it does not belong to.

- `/_views/policy/POL-1@published` lists both CTL-1 and CTL-3, although only the draft face's content cites CTL-1.
- `CTL-1`'s page in the `published` world shows POL-1 as an incoming neighbour, also to a reader holding only `policy@published`. That reader learns what the draft cites, which is draft content.
- The server logs `dataentry: world neighbors: edge names neither endpoint; dropped` (`internal/dataentry/worldneighbors.go:301`).

Likely cause, not yet verified: `worldEdgesForWire` keeps an edge when its head
resolves in the world and is readable (`headOnWire`), but never checks that a
content-scoped edge's tail face (`FromFace`) is the face served for that source.
The warning suggests `selfID` is sometimes an `ID@face` string compared with
bare ids.

## Expected

A content-scoped edge appears only with the face that owns it: on the source's
page when that face is served, and as an incoming neighbour only when the world
resolves the source to that face and the reader may read it. Identity-scoped
edges are unaffected.
