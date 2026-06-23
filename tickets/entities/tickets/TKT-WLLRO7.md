---
id: TKT-WLLRO7
type: ticket
title: 'Configurable per-property attachment count: file property `max` setting (1..N)'
kind: enhancement
priority: medium
effort: l
status: done
---

## Description

Move attachments from **1:1 per property** to **configurable N-per-property**
via a `max` setting on the metamodel `file` property. Use cases like "a few
supporting PDFs on a decision" need N-per-property. Sequenced **before** the
feature ships externally so the wire contract is locked once (nothing is
committed yet — no back-compat constraint).

## Locked design (research-backed — see RES findings on the ticket)

**Identifier = filename-as-key.** Store key `(entityID, property, fileName)`.
The research established that an opaque/UUID id buys **no security** here —
access is gated by the *owning entity's* read permission (re-checked on every
byte request, with the RR-NGMI indistinguishability invariant), so a guessable
attachment id leaks nothing you weren't already handed in `_attachments` and
can't reach an entity you can't read. Filename-as-key also gives the cheapest
migration (filename is already the on-disk leaf + the pgstore `file_name` column
→ migration is "add file_name to the PK", forward-only/idempotent).

**Normalization + auto-suffix.** Filenames are normalized to a safe key
(reuse/extend `safeAttachmentFilename` + `storeutil`). On collision within a
property (same normalized name, under `max`), **auto-suffix** like a file
manager: `file.png` → `file (1).png` → `file (2).png`. Never lose a file.
Consequence: there is no separate "replace" verb — every upload adds (suffixing
on collision) or hits the `max` ceiling; "replace" in the UI = remove-then-add.

**Wire shape = always-list.** `_attachments[property]` becomes an **array** of
`V1Attachment` even for `max:1` (a 1-element array). This matches rela's
established convention (`list: true` properties and `_relations` are always
arrays, never scalar-when-one) and avoids the polymorphic `Array.isArray` branch
at all 8+ frontend consumer sites + the shape-flip footgun when `max` changes.
`V1Attachment` gains an `id` field (the normalized filename) so per-file
URLs/delete can target it.

### Metamodel
- `file` property gains optional `max` (default 1). Validate `max >= 1`. Document in `docs/metamodel.md`.

### Store (3 backends + pg migration)
- Key `(entityID, property, fileName)`. fsstore: `attachments/<id>/<prop>/<fileName>` (already the layout — index key gains the filename). memstore: map key `entityID/property/fileName`. pgstore: PRIMARY KEY → `(entity_id, property, file_name)`, forward-only migration.
- New/updated `AttachmentManager` surface: `AttachFile` appends (no longer overwrites by property); add the ability to read/delete a **specific** `(entityID, property, fileName)`; `ListAttachments` already returns per-file rows. Keep cascade-on-delete/rename working. Pass `internal/store/storetest` (new cases: N-per-property, suffix-on-collision, per-file delete, cascade).
- The entity property value becomes a **list** of filenames (or paths) when populated. Decide the exact stored representation in planning (list of filenames vs list of path strings) — keep it consistent with `entity.SetString`/list handling.

### API (always-list + per-file ops)
- `_attachments[property]: []V1Attachment` with `id` (normalized filename). `computeAttachments` appends per file.
- Upload `PUT/POST /_attachments/{property}` appends up to `max` (auto-suffix on name collision); rejects with a clear error (409/422) when at `max`.
- Per-file download + delete: `GET|DELETE /_attachments/{property}/{fileName}`. Bare `/_attachments/{property}` GET can 404 or list; delete of a specific file by id. Keep the up-front `update`-authorize + read-gate + uniform-404 invariants from TKT-RXFD5B.
- Update `docs/data-entry/api-reference.md`.

### CLI
- `rela attach` appends up to `max` (auto-suffix on collision); errors when over. `rela detach` disambiguates by filename when a property holds more than one.

### Frontend (multi-file widget)
- FileWidget renders a **list**: per-file preview (images)/download/remove; an add control (picker + drag-drop) disabled at `max`. Thread the array through PropertyDisplay / FieldRenderer / SectionEditForm / DynamicForm (all currently single-`AttachmentInfo`). Update widget + form tests.

### Acceptance
- A `file` property with `max: 3` holds up to 3 files; the 4th is rejected clearly. Uploading a duplicate name auto-suffixes.
- `max: 1` (default) behaves like today except the wire is a 1-element array.
- storetest conformance passes on all backends; pg migration forward-only + idempotent; existing single-attachment data migrates cleanly.
- Web: add/preview/download/remove multiple files; add disabled at max.

### Decisions (user)
- Filename-as-key (not UUID — no security benefit given the entity ACL). Normalization required.
- Auto-suffix on same-name collision.
- Always-list wire shape.

Parent: FEAT-870YCY.
