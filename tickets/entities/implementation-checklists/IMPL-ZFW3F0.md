---
id: IMPL-ZFW3F0
type: implementation-checklist
title: 'Implementation: Attachment web write path: upload endpoint + drag-drop widget + default size limit + Lua file info'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests for new code — `handlers_attachment_write_test.go` (upload/replace/delete/denied/oversize/bad-field); storetest `RejectsOversize` conformance (all backends); frontend FileWidget upload + error tests
- [x] Integration tests — handler-level via gate ctx; round-trip upload→GET; router_walk probe
- [x] Happy path — upload (PUT/POST), delete (DELETE), drag-drop/picker widget, configurable size cap
- [x] Edge cases — missing file field (400), non-file property (404), oversize (413), locked entity (422), idempotent delete
- [x] Error handling — store/gate errors mapped, logged, not leaked

## Test Quality

- [x] Fixture builders (newTestAppV1, seedEntity, multipartBody, writeACL)
- [x] No hardcoded values where object in scope
- [x] Only test-relevant values specified
- [x] Property comparisons use seeded objects

## Manual Verification

**Verification Evidence:**
- Backend: `go test ./internal/dataentry/ ./internal/store/... ./internal/dataentryconfig/ ./internal/attachment/` green; `go build` default+postgres; `golangci-lint` 0 issues; `just arch-lint` OK; coverage PASS.
- Frontend: `npm run test:run` 1086 passed; `vue-tsc` clean; `npm run lint` 0 errors.
- AC1 upload/replace/view/delete: `TestAttachmentUpload_RoundTrips`/`_Replaces`/`TestAttachmentDelete_RemovesBytesAndProperty` PASS (fsstore; pgstore conformance via storetest).
- AC2 over-limit all backends: handler `TestAttachmentUpload_TooLarge` (413) + storetest `RejectsOversize` (fs/mem/pg, sentinel + no partial) PASS.
- AC3 no-update-access denied before bytes: `TestAttachmentWrite_DeniedBeforeBytes` (bob→404 read-gate, carol→403, no bytes written) PASS.
- AC4 orphan window closed: authorize-before-write (preflight) + delete clears property first; covered by the denied-before-bytes assertion.

## Quality

- [x] Follows patterns — mirrors authorizeConflictResolve (translateVerb, no direct WriteRequest → lint_test green), theme multipart, writeV1Error problem+json, closed-world DTO
- [x] DRY — shared `attachmentWritePreflight`; shared `storeutil.LimitAttachmentReader` across fs/mem + `store.MaxAttachmentBytes`/`ErrAttachmentTooLarge` sentinel across all 3 backends
- [x] No security issues — update-ACL re-authorized server-side before bytes; ingress + per-store size caps; uniform 404; content-cap reader independent of multipart envelope; no error leak
- [x] No silent failures — errors logged AND returned
- [x] No debug code left behind

## Scope change (recorded)
The "Lua file-info veto" was split to **TKT-40PZ15** (net-new write-veto
mechanism, not a binding addition — established by research). Size-limit seam =
ingress + per-store backstop (user decision).
