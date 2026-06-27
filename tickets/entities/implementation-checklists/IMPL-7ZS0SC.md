---
id: IMPL-7ZS0SC
type: implementation-checklist
title: 'Implementation: Configurable per-property attachment count: file property `max` setting (1..N)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests for new code across all layers — metamodel (FileMax + list validation), storetest conformance (append/same-name-replace/per-file-delete/suffix-key/rename-all/oversize-replace), handler (multi-append/409-cap/auto-suffix/per-file-delete/list-shape), widget (list render/replace-vs-add/at-cap/remove/upload/progress)
- [x] Integration: live end-to-end smoke on the demo app (3-upload+409, auto-suffix, per-file download/delete, single-cap scalar)
- [x] Happy path + edge cases (cap, collision, rename, oversize-replace) + error handling

## Test Quality

- [x] Fixture builders; multi-file fixture (`docs` max:3) added to newTestAppV1; no hardcoded values where object in scope

## Manual Verification

**Verification Evidence (live, demo app NOTE-001):**
- gallery (max:3): 3 uploads → 200; 4th → 409; `_attachments.gallery` is a 3-element list with per-file ids+hrefs; `properties.gallery` is a list.
- Auto-suffix: re-uploading `g1.png` → stored as `g1 (1).png` (no overwrite).
- Per-file delete (204) + per-file download (correct bytes).
- attachment (max:1): replace mode, `properties.attachment` stays a scalar string, `_attachments` is a 1-element list.
- Backend: `go build` default+postgres; `go test ./...` green; `golangci-lint ./internal/...` 0; `just arch-lint` OK.
- Frontend: `npm run test:run` 1089 passed; `vue-tsc` clean; `npm run lint` 0 errors.

## Quality

- [x] Follows patterns — store key extended like the existing index key; filename helpers moved to `store` (not storeutil) to respect the dataentry→store arch boundary; always-list matches `_relations`/`list:` convention; write-path max enforcement mirrors the read/write ticket structure
- [x] DRY — single `store.CapAttachmentReader`/`NormalizeFileName`/`SuffixOnCollision`/`ValidateFileName`; shared resolve/stamp logic in handler + service
- [x] No security issues — entity-ACL gate preserved on per-file routes; `ValidateFileName` rejects path separators in the new key segment; auto-suffix prevents silent loss
- [x] No silent failures; no debug code

## Decisions (user + research)
Filename-as-key (UUID rejected — no security benefit given the entity ACL);
auto-suffix on collision; always-list wire shape; store stays max-agnostic (max
enforced in write path); widget max-aware (replace at 1 / add at N); progress
via axios onUploadProgress (no lib).
