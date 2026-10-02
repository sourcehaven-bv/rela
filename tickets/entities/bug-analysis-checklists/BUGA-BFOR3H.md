---
id: BUGA-BFOR3H
type: bug-analysis-checklist
title: 'Analysis: Entity export 404s for every faced address, while the detail page still offers the Export menu'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally: a scratch test on memstore gives `TKT-1` export 200, `TKT-1@published` export 404, and `/_documents/report/TKT-1@published` 500 ("computing document hash: entity \"TKT-1@published\" not found").
- [x] Minimal reproduction steps documented (bug body).
- [x] Environment/conditions noted: every backend for the export 404. The document 500 comes from `store.GetEntity` receiving an address; on pgstore it is a guaranteed miss.

## Root Cause

- [x] Immediate cause identified (why1): the explicit non-default-face guard in `handleV1ExportEntity`.
- [x] Contributing factors found (why2-3): the guard was correct when written (#1527). `PolicyReader.Get` became address-aware and face-gated in #1682, and the guard was not revisited.
- [x] Systemic cause explored (why4-5): deferred work was recorded as a comment citing an unrelated ticket. The address-parse measure does not catch a route that parses and then refuses a face, or that passes the address on as a bare id.

## Fix Planning

- [x] Fix approach determined:
  1. `export.go`: remove the default-face guard. Resolve the row with `visibleReader.getVisibleRef` (world resolution, world deny, row and face gate, the same path as the detail GET). Redact it once before rendering.
  2. `exportRenderer`: validate the address with `isSafeStateRefSegment` instead of `isSafePathSegment`, and pass the address as the entry id, so `export_render` gets `entity.face` and `rela.document.entry_id`.
  3. `document.go`: parse the entry address in `computeDocumentHash` and `renderCommand` and read with `GetEntityState`.
  4. SPA `EntityDetail.vue`: build the export URL from `servedRef`, not the route `entityId`.
  5. Replace the TKT-5SZG2L comment with one that describes the faced behaviour.
- [x] Regression test planned: assert WHICH face was exported (body content, per BUG-VFHUWO's prevention), not only the status. Cases: faced export with the built-in renderer, faced export with `export_render` receiving the address, `ticket@published` caller asking for a draft face gets a 404 with a positive control, and faced anchored document render and export. Add a pgstore-gated case for `documentService`, because memstore resolves suffixed ids by accident. Add a Vitest case that the export URL carries the served face.
- [x] Related areas checked for similar issues: the document routes skip the face gate entirely (BUG-6DBV6N, filed separately, high). List export reads headers through the gated list path and is not affected.
