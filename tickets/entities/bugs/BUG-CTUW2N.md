---
id: BUG-CTUW2N
type: bug
title: 'Faced types: attachment upload/download and entity export 404 on ID@face; file bytes shared across faces'
description: On a faced type every entity exists only as ID@face. Attachment PUT/DELETE/GET strip the face and load a bare row that does not exist (404); entity export refuses ID@face (404); and the attachment service stamps a face's file property from the per-entity file listing, so faces sharing bytes overwrite and delete each other's files.
priority: high
effort: l
why1: The attachment routes call bareEntityID and the preflight loads the bare row with entityReader.getEntity; export rejects any address whose face is not the default. On a faced type that bare row does not exist, so all three answer 404.
why2: TKT-SLFURL (#1527) decided attachments and export are per entity and served the bare face. At that time a faced type still had a bare face (bare_face), so the decision produced a working if face-blind route.
why3: 'BUG-HC6I2T (#1557) removed bare_face: a faced type stores no zero-coordinate row. It changed what a bare id means but did not revisit the callers that relied on the bare row existing (bareEntityID callers, export guard). The export comment still points at TKT-5SZG2L, which is done and never recorded the gap.'
why4: No test drives an attachment or an export on a faced type. The attachment tests and e2e/tests/attachments-create.spec.ts use faceless types, so the 404 was invisible to CI. The attachment service also models files per entity (listing + stampValue), a model that only holds while an entity has one row.
why5: 'Systemic: face support is added per route with no inventory of which features must be face aware and no shared test matrix. A change to face semantics (bare_face removal) cannot find its affected callers, and each face-blind surface is found by a user. This is the fifth occurrence of the shape (BUG-64MU2Q, BUG-OOZBBK, BUG-VFHUWO, BUG-R1PQY9).'
status: ready
---

## Problem

A faced type stores no row at the bare coordinate (BUG-HC6I2T). Every entity of
such a type exists only as `ID@face`. Three attachment/export surfaces do not
handle that.

1. `PUT|DELETE /api/v1/{plural}/{id}@{face}/_attachments/{property}[/{file}]` strips the face (`bareEntityID`, `api_v1.go:245,261`). `attachmentWritePreflight` then loads the bare row (`h.reader.getEntity`), which does not exist, and answers 404. A file property on a faced type can never be filled through the web app.
2. `GET .../_attachments/{property}/{file}` has the same shape: it loads the bare row and applies `faceReadable` to that row, so it 404s as well.
3. `GET /api/v1/{plural}/{id}@{face}/_export?transform=...` refuses any faced address (`export.go:210`). The comment cites TKT-5SZG2L as the follow-up, but that ticket is done and does not record the gap.
4. The attachment service derives a property value from the per-entity file listing (`attachmentFileNames` + `stampValue`), not from the face's own value. Once faces share one byte directory, an upload on one face stamps files that belong to another, and a single-file replace deletes bytes another face still references.

## Reproduce

Scratch test (kept in `.ignored/repro_faced_attach_test.go.txt`): type `policy`
with faces `draft`, `published` and a `file` property; seed `POL-1@draft` only.

- `GET /api/v1/policys/POL-1@draft` → 200
- `PUT /api/v1/policys/POL-1@draft/_attachments/doc` → 404 "Entity not found"
- `GET /api/v1/policys/POL-1@draft/_export?transform=md` → 404

## Expected

- Upload, delete and download resolve the addressed face. The property value lives on that face; the ACL decision is `update` (or read) on `type@face`.
- Export accepts `ID@face` and exports that face.
- File bytes may stay keyed per entity id. A face's property value is computed from that face's own value, and bytes are removed only when no face of the entity still references them (e.g. after a copy with `fields: all`).

## Fix plan

1. **Routes.** The `_attachments` routes parse the address with `parseEntityRef` instead of `bareEntityID`. The write preflight loads the row with `getEntityRef`; GET loads it the way `getVisibleRef` does (bare-id row gate, then face gate). The existing `update` decision then keys on the right `type@face`.
2. **Export.** Drop the default-face guard in `handleV1ExportEntity` and pass the address to `visReader.Get`, which already parses `ID@face`. Thread the face into `exportRenderer` so an `export_render:` document renders that face.
3. **Attachment service.** Bytes stay keyed per entity id. A face's value is computed from that face's own property value, not from the directory listing. New file names are resolved against the files referenced by every face, so two faces never overwrite each other's bytes. Replace and delete remove bytes only when no other face still references them. The CLI `attach`/`detach` accept `ID@face`.
4. **Regression tests.** Handler tests on `facedApp`: upload, download, delete and export on `POL-1@draft`, and a `type@face` grant that allows draft updates but not published ones. Service test: copy with `fields: all`, then replace and delete on one face; the other face still downloads. One Playwright spec on a faced fixture project covering upload, copy and replace.
5. **Measure.** A guard test that fails when a dataentry sub-resource handler reads by bare id (`bareEntityID`, `entityReader.getEntity`) on an addressed route.

## Related areas

Checked with a full face-awareness inventory
(`.ignored/face-awareness-inventory.md`). In scope because they share the
per-entity file model: the MCP `list_attachments` and `read_attachment` tools
list and open files per entity (`mcp/tools_attachment.go:345,429`), so a reader
of one face sees files uploaded to another. The other gaps are filed separately:
BUG-1YN750, BUG-8J3LSB, BUG-4SYAA6, BUG-BZQQDP, BUG-J3PBFN, BUG-95W7MV,
BUG-7MB1D5, BUG-FYEEVX and TKT-WCMW47.
