---
id: IMPL-RF6G6C
type: implementation-checklist
title: 'Implementation: Attachment web read path: ACL-gated download endpoint + file widget/preview'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `handlers_attachment_test.go` (6 backend cases); `widgets.test.ts` FileWidget (4 cases) + updated registry case
- [x] Integration tests written — handler-level tests invoke through the gate ctx (package convention); router_walk probe pins route registration
- [x] Happy path implemented — GET bytes, `_attachments` metadata, FileWidget display + image preview
- [x] Edge cases handled — denied(404)/missing/wrong-property/non-file/rename/method-not-allowed; widget fallback to raw path + empty-state
- [x] Error handling in place — store/gate errors mapped via writeV1Error/writeGateError, logged server-side, not leaked

## Test Quality

- [x] Fixture builders used (`seedEntity`, `seedAttachment`, `mustNewACL`, shared `newTestAppV1`)
- [x] No hardcoded values where object in scope (e.g. `att.Size` compared to `len("bytes")`)
- [x] Only test-relevant values specified
- [x] Interpolated values constructed from objects
- [x] Property comparisons use the seeded object

## Manual Verification

**Verification Evidence:**
- Backend: `go test ./internal/dataentry/` green; full store suite green; `go build` default+postgres OK; `golangci-lint` 0 issues; `just arch-lint` OK.
- Frontend: `npm run test:run` 1080 passed; `vue-tsc` typecheck clean; `npm run lint` 0 errors (warnings all pre-existing, none in changed files).
- AC1 (allowed→bytes+headers): `TestAttachment_AllowedReturnsBytes` PASS.
- AC2 (denied→404 parity, no ETag): `TestAttachment_DeniedIsNotFound` PASS — required aligning all handler 404s to the gate's "Entity not found" title (RR-NGMI indistinguishability); captured as `notFoundTitle` const with a comment.
- AC3 (missing/unknown/non-file→404): `TestAttachment_UnknownOrNonFileProperty` + nonexistent case PASS.
- AC4 (rename resolves by current id): `TestAttachment_ResolvesByCurrentIDAfterRename` PASS.
- AC5 (SPA preview/download): FileWidget unit tests PASS (image preview, non-image download, raw-path fallback, empty-state). Live SPA smoke deferred to review.
- `_attachments` present on per-entity GET, absent on list rows: `TestAttachment_MetadataOnEntityGET` PASS.

## Quality

- [x] Follows project patterns — mirrors `_actions` route case, theme serve-bytes headers, `_fields`/`_relations` closed-world DTO, acl_get_test harness
- [x] DRY — `notFoundTitle` const; `contentTypeForFilename`/`safeAttachmentFilename` helpers; reuses `unsafeFilenameRe`. Did not over-extract.
- [x] No security issues — ACL gate before store touch; nosniff+CSP sandbox+Content-Disposition on user bytes; filename sanitized; entity-first resolution (no path traversal); no error-string leak
- [x] No silent failures — list-attachments failure in the metadata path degrades to "no attachments" (UI hint only); the bytes endpoint still gates+serves correctly
- [x] No debug code left behind
