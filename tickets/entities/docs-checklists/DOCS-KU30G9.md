---
id: DOCS-KU30G9
type: docs-checklist
title: 'Docs: Attachment web read path'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new handler (`handleV1GetAttachment`), helpers, and the `V1Attachment` DTO / `_attachments` field — including the ACL-inheritance, entity-first-resolution, and serve-defensive-bytes rationale
- [x] ~~CLAUDE.md pattern update~~ (N/A: reuses existing patterns — `_actions` route case, closed-world DTO, gate-before-load — no new convention)

## Project Documentation

- [x] `docs/data-entry/api-reference.md` — added the **Attachments (`_attachments`)** section: metadata shape on per-entity responses, the `GET /{plural}/{id}/_attachments/{property}` download endpoint, ACL-inheritance + 404-not-403 behavior, and the security headers (nosniff / CSP sandbox / Content-Disposition)
- [x] Documented closed-world semantics (rides every per-entity response, absent on list rows) consistent with `_fields`/`_relations`

## External Documentation

- [x] ~~README / external docs~~ (N/A: internal API surface, covered by the API reference)

**Docs verified:** the api-reference Attachments section matches the implemented
endpoint path, headers, and payload field names
(`filename`/`size`/`contentType`/`href`).
