---
id: BUG-PLZDPR
type: bug
title: Entity export 404s for every faced address, while the detail page still offers the Export menu
description: GET /{plural}/{id}/_export answers ID@face with 404 and the SPA still shows the Export menu on faced detail pages; faced entity-anchored documents fail with 500 because documentService reads the address as a bare id.
priority: medium
effort: m
why1: 'handleV1ExportEntity returns 404 for any address whose face is not the default before it reads anything. The SPA shows ExportMenu on every detail page and builds its URL from the route entityId instead of servedRef. Faced documents fail later: documentService passes the whole address to store.GetEntity (bare id by contract) in computeDocumentHash and renderCommand.'
why2: When TKT-SLFURL made the face part of the address (#1527, 2026-09-08), the export route's reader (visibility.PolicyReader.Get) read by bare id only. The guard stopped it serving the bare face under a faced name. The follow-up was attributed to TKT-5SZG2L instead of getting a ticket of its own.
why3: PolicyReader.Get became address-aware and face-gated in BUG-R1PQY9 (#1682, 2026-09-28), but nothing linked the export guard to that reader. The guard and its comment stayed, and no test asserts that a faced export succeeds.
why4: The deferred work lived only in a code comment that cited a ticket whose scope did not include it, so closing that ticket closed nothing. The SPA gates the Export menu on transform availability only, not on whether the server can export the served face.
why5: Faced addressing is rolled out route by route. The faced-route-address-parse-test measure checks that a route parses the address. It does not catch a route that parses and then refuses a face, or that passes the parsed string downstream as a bare id. Deferred gaps are recorded as prose instead of as tickets.
prevention: Router-level tests pin that export and anchored documents serve the addressed face, honour the world, and face-gate neighbors. The deferred work is now tracked by ticket rather than a code comment. A route-registry test asserting every address-accepting route serves and gates faces would close the class (see BUG-6DBV6N).
status: done
---

## Problem

`GET /api/v1/{plural}/{id}/_export?transform=<name>` refuses every address with
a non-default face. `handleV1ExportEntity` in `internal/dataentry/export.go`
answers `ID@face` with the not-found a missing entity gets. Its comment cites
TKT-5SZG2L, which is done and does not mention export, so the gap was untracked.

The SPA still renders the Export menu on a faced detail page, and every choice
returns 404. For a type that declares faces, nothing is stored at the bare
coordinate (BUG-HC6I2T), so no address of that type can be exported.

The SPA also builds the export URL from the route's `entityId`, not from
`servedRef`. Under a world the route id is bare while the page shows a
world-resolved face, so the export would target the wrong row even once the
handler accepts faces.

Entity-anchored documents share the gap by a different route. Since BUG-VFHUWO,
`/_documents/{doc}/{id}` accepts `ID@face`, but `documentService` passes the
whole address to `store.GetEntity` (bare id by contract) in
`computeDocumentHash` and `renderCommand`. A faced document render fails with
500 "computing document hash: entity \"TKT-1@published\" not found".

## Reproduce

1. Declare a type with `faces: [concept, approved]` and a registered `pdf` transform.
2. Open `/entity/<type>/X-001@concept` and choose Export › PDF.
3. `/api/v1/<plural>/X-001@concept/_export?transform=pdf` returns 404.

Unit repro (memstore): `TKT-1` exports with 200, `TKT-1@published` returns 404.
`/_documents/report/TKT-1@published` returns 500.

## Expected

The export renders the face the page shows, with the same gates as the faced
detail read: the ACL read gate for that face, field redaction, and the
face-aware world resolution. An `export_render` override receives the face as
`entity.face` and the address as `rela.document.entry_id`.

## Until fixed

Hide the Export menu on detail pages whose served face is not the bare face.

## Acceptance criteria

1. Exporting `ID@face` returns that face's content, rendered by the built-in renderer or by `export_render`.
2. A caller who may read the bare face but not the requested face gets the same 404 as for a missing entity.
3. The code comment names the ticket that actually tracks this.
4. An entity-anchored document renders and exports for `ID@face`.
5. The SPA export URL uses the served address.
