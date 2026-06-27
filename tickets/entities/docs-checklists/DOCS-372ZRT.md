---
id: DOCS-372ZRT
type: docs-checklist
title: 'Docs: Multi-attachment per property'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Godoc on the new surface — `PropertyDef.Max`/`FileMax()`, store `ValidateFileName`/`NormalizeFileName`/`SuffixOnCollision`, `attachment.Service.WriteAttachment`/`DeleteAttachment`/`ErrAtCapacity`, the per-file handler routes, `V1Attachment.id`, the always-list `_attachments`

## Project Documentation

- [x] `docs/metamodel.md` — `max` property option + "File attachments and `max`" section (single vs multi, list value, auto-suffix)
- [x] `docs/data-entry/api-reference.md` — Attachments section rewritten for the always-list `_attachments` shape (+ id), per-file download/delete routes (`/_attachments/{property}/{fileName}`), upload max-behavior (replace at 1 / append+suffix+409 at N)
- [x] `docs/cli-reference.md` — `rela attach` max-aware note + `rela detach --file` flag

## External Documentation

- [x] ~~README~~ (N/A: covered by metamodel + API + CLI references)

**Docs verified:** the api-reference list shape, per-file URLs, and
409/auto-suffix behavior match the implementation and the live smoke test.
