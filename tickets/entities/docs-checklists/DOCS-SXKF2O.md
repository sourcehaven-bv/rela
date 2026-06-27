---
id: DOCS-SXKF2O
type: docs-checklist
title: 'Docs: Attachment web write path'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new handlers (upload/delete/preflight), the shared `store.CapAttachmentReader`, the `ErrAttachmentTooLarge`/`MaxAttachmentBytes` sentinels, and `AppConfig.MaxAttachmentBytes` — including ACL/orphan-ordering and size-cap rationale
- [x] ~~CLAUDE.md pattern update~~ (N/A: reuses existing write patterns — translateVerb authorize, multipart, closed-world DTO)

## Project Documentation

- [x] `docs/data-entry/api-reference.md` — Attachments section extended with the **Upload** (PUT/POST multipart) and **Delete** (DELETE) endpoints: `update`-permission inheritance, authorize-before-bytes, 413 size limit + the `app.max_attachment_bytes` config key + per-store backstop, 400/422 error cases
- [x] Documented orphan-avoidance ordering (delete clears property before bytes)

## External Documentation

- [x] ~~README / external docs~~ (N/A: internal API surface, covered by the API reference)

**Docs verified:** api-reference upload/delete sections match the implemented
methods, field name (`file`), status codes, and the config key name
(`app.max_attachment_bytes`).
