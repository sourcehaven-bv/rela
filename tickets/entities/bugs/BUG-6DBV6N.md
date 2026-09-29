---
id: BUG-6DBV6N
type: bug
title: 'Entity-anchored documents skip the face gate: a face-restricted reader can render a hidden face'
description: resolveAnchoredDocument applies only the face-blind row gate and never faceReadable, so a principal with a type@published grant can render a document (and a command renderer receives the raw entity) for a face they may not read.
priority: high
effort: s
why1: resolveAnchoredDocument ran only the face-blind row gate on the bare id and then read GetEntityState(id, face) without calling faceReadable.
why2: The anchored-document routes accepted an address (BUG-VFHUWO) after TKT-O7R2A1 had added face gating route by route to the entity GET, view entry and relations routes, so they were not in that sweep.
why3: Face gating is a per-handler call rather than a property of the reader these routes use, so each new faced route must remember to add it.
why4: No test enumerated every route that accepts an address and asserted the face gate on it; facegate_surfaces_test.go listed surfaces by hand.
why5: Faced addressing is rolled out route by route without a registry of address-accepting routes that a single test could iterate.
prevention: TestFaceGrant_AnchoredDocumentIsFaceGated pins both document routes. The faced-route-address-parse-test measure covers parsing; a route-registry face-gate test would close the class.
status: done
---

## Problem

`resolveAnchoredDocument` in `internal/dataentry/export_document.go` applies
only the face-blind row gate (`gateReadOrNotFound` on the bare id). It then
reads `store.GetEntityState(ref.ID, ref.Face)` and never calls `faceReadable`.
Both `/_documents/{doc}/{id}` and its `_export` route use it.

A principal whose grant is `ticket@published` can therefore run a document
render against a face they may not read. A `command:` renderer receives the raw,
unredacted entry entity as `{in}` (`documentService.renderCommand` reads
`store.GetEntity` directly), so the hidden face's content reaches the output. A
`script:` renderer reads through the visibility reader, but it still runs and
the response lists the entry id in `entity_ids`.

TKT-O7R2A1 added the face gate to the entity GET, the view entry, and the
relations route. `facegate_surfaces_test.go` added it to attachments and
history. The document routes were not covered.

## Reproduce

Using `seedDraftAndPublishedTicket` and `publishedOnly`, request
`documentsAs(aliceCtx(), ..., "report", "TKT-1")`. The response is 200, and the
fake engine records a render of `TKT-1`, whose only readable face for alice is
`published`.

## Expected

A face the caller may not read gets the same 404 as a missing entity, and the
renderer does not run.

## Acceptance criteria

1. `resolveAnchoredDocument` applies `faceReadable` to the row it loads, before any render.
2. A test in `facegate_surfaces_test.go` pairs the denial with a positive control, for both the HTML render and the export route.
