---
id: PLAN-ZFQPKQ
type: planning-checklist
title: 'Planning: Configurable per-property attachment count: file property `max` setting (1..N)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/scope/acceptance — see ticket. N-per-property via metamodel `max`; locked design below.

**Locked decisions (user + research):**
- **Filename-as-key** `(entityID, property, fileName)` — research: opaque/UUID id buys no security (entity ACL gates access), worse migration. Normalization required.
- **Auto-suffix** on same-name collision (`file.png`→`file (1).png`).
- **Always-list** wire shape (`_attachments[property]: []V1Attachment` with `id`), matching rela's `list:`/`_relations` convention.
- **Store stays max-agnostic**: pure append + per-file delete. **Max-enforcement in the write path**: `max==1` → delete-then-attach (replace); `max>1` → append up to max, 409 at cap.
- **Widget max-aware**: `max==1` replace mode; `max>1` add/list mode.
- **Upload progress** via axios `onUploadProgress` (existing dep, no lib).

## Research / blast radius (from sub-agent map)

- `store.AttachmentManager`: `ReadAttachment` + `DeleteAttachment` gain `fileName` param (store.go:268-269). `AttachFile` already has it but stops overwriting-by-property. `ListAttachments`/`AttachmentInfo` already per-file.
- Backends: fsstore index key `entityID+"/"+property` → +`/"+fileName` (attachment.go:41/108/124 + renameAttachmentDir re-key:245); memstore same (684/698/710 + rename:475); pgstore PK `(entity_id,property)`→`(entity_id,property,file_name)` via new migration `0003_*.sql`, `ON CONFLICT` + 2 WHERE clauses (55/66/80), drop `file_name DEFAULT ''`. Cascade (removeAttachmentDir/DeleteEntity loops) is key-agnostic — OK.
- Entity: no `SetStringList`; assign `[]string` directly to `e.Properties[prop]` (entity.go) or add a setter. Stamp list of filenames/paths.
- Metamodel: add `Max int yaml:"max,omitempty"` to PropertyDef (types.go:177); validation.go:341 file case grows a list branch gated on List/Max; validatePropertyDefs rejects Max<1.
- Shared filename normalize + collision-suffix helper → `internal/store/storeutil` (already imported by all 3 backends); add `ValidateFileName`.
- Call sites: dataentry handlers_attachment.go (108/218/230/265/275), affordances.go computeAttachments (852-871 → append per file), cli/detach.go (37-40), attachment/attachment.go (107-112).
- storetest: rewrite `OverwritesExisting`→append, `OversizeReplaceKeepsExisting`, add filename arg everywhere; new cases: N-per-property, suffix-on-collision, per-file delete, ValidateFileName. Also differential_test.go:201, fuzz.go:100/109, pgstore/fsstore backend tests.
- Frontend consumers of `_attachments` (single→array): entity.ts:34 type, EntityDetail:438/815, DynamicForm:273/1114/1156, SectionEditForm:46/219/233, FieldRenderer:23, PropertyDisplay:24/97, widgets/types.ts:54, FileWidget. Plus widget tests.
- Progress: api/attachments.ts uses fetch (no progress) → switch to axios client (`onUploadProgress`).

## Approach (layered)

1. **Metamodel**: `Max int` on PropertyDef; validate `max>=1`; file validation accepts list when max>1; docs/metamodel.md.
2. **Store**: storeutil normalize+suffix+`ValidateFileName`; `ReadAttachment`/`DeleteAttachment` gain fileName; 3 backends per-file key; pg migration `0003`; rewrite storetest + backend tests.
3. **Service/handlers**: write-path max enforcement (delete-then-attach at 1, append+suffix+409 at N); per-file routes `GET|DELETE /_attachments/{property}/{fileName}`; affordances appends per file; stamp list property.
4. **API DTO**: `_attachments[property]: []V1Attachment` + `id`; docs.
5. **CLI**: attach append-up-to-max; detach by filename.
6. **Frontend**: array type + thread through 4 layers; max-aware FileWidget (replace vs add/list, per-file remove, add disabled at max); axios upload + progress bar; tests.

## Security / Test / Risk

- Security: same entity-ACL gate (per-file routes keep up-front authorize + uniform 404). `ValidateFileName` closes path-key injection (filename now a storage-key segment). Auto-suffix prevents silent loss.
- Tests: storetest conformance (all backends incl pg migration idempotent), handler upload/replace/append/max-cap/suffix/per-file-delete, widget max==1 vs max>1, progress.
- Risks: pg migration over existing `DEFAULT ''` rows — migration must handle; storetest semantics inversion (overwrite→append) — rewrite carefully; broad frontend thread — array everywhere, one shape.

## Design Review

- [x] ~~/design-review~~ (N/A: design researched + user-decided on every fork: id scheme, suffix, wire shape, max-enforcement location, widget mode, progress. Cranky /code-review at review stage.)
