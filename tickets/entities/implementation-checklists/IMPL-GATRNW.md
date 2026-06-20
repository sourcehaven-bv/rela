---
id: IMPL-GATRNW
type: implementation-checklist
title: 'Implementation: Sync 4/5: server sync HTTP API (manifest + conditional push) on data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written — internal/dataentry/sync_test.go
- [x] Integration tests — full-router requests through the security middleware
- [x] Happy path — manifest / content GET / conditional PUT (200/412/422) / DELETE, under /api/sync/
- [x] Edge cases — first-create no-If-Match, blind-push 412, base-less delete 412, unknown-type 422, path-traversal reject, manifest cursor advance, 501 on non-pg backend
- [x] Error handling — 412 (stale) vs 422 (invalid) distinct; ACL 403; ErrEntityNotFound 404

## Test Quality

- [x] Fixture builders — newHandlerTestApp / buildAppWithACLAndAudit / manifestStore wrapper
- [x] No hardcoded values when object in scope
- [x] Only values that matter
- [x] Interpolated from objects
- [x] Comparisons use original object

## Manual Verification

- [x] End-to-end via the full router — `go test -race ./internal/dataentry/` green
- [x] Each acceptance criterion verified
- [x] Edge cases verified

**Verification Evidence:**
- `go test -race ./internal/dataentry/` ok; `go test -tags postgres ./internal/store/pgstore/` ok (ManifestSince type move); all 3 build tags compile; pgx not in default rela-server; bleve not in pgstore; arch-lint clean; lint clean.
- AC manifest returns changed + tombstones keyed by cursor: TestSync_ManifestSerialization (live + tombstone + relation key + cursor advance).
- AC push 200/412/422 distinct: TestSync_PushUpdate_HappyAnd412, TestSync_PushCreate, TestSync_PushCreate_NoIfMatchOnExisting (412), TestSync_Push422_UnknownType.
- AC audit attributes Tool=sync + proxy principal: TestSync_AttributesToToolSync.
- AC /api/sync reachable by no-Origin client but Host-checked; path-traversal rejected; malformed cursor → full manifest: TestSync_SameOriginExemption, TestSync_PathTraversalRejected.

## Quality

- [x] Follows project patterns — writeV1JSON/writeV1Error; a.writeMu for writes; consumer-side interfaces (manifestProvider/syncApplier) at the call site per CLAUDE.md
- [x] DRY — shared preconditionOK / deletePreconditionOK / writeSyncApplyError
- [x] No security issues — see code review below
- [x] No silent failures
- [x] No debug code

## Code Review (cranky-code-reviewer)

Found 1 critical (CSRF) + 3 significant/minor, all addressed:
- **RR-X869DY (CRITICAL)** — the blanket /api/sync/ same-origin exemption was CSRF-exploitable under a cookie-mode OAuth proxy. FIXED: exemption is now Origin/Cookie-aware (isCSRFExempt) — only a no-cookie, no-origin request (provably non-browser) skips same-origin; a cookie-bearing/cross-origin request is rejected. Regression TestSync_CSRFExemptionRequiresNoCookie.
- **RR-9RHEWN (sig)** — relation manifest key `--` collided with path `/` and was ambiguous. FIXED: unified to slash form.
- **RR-2D1SZO (sig)** — base-less DELETE was a blind delete. FIXED: requires matching If-Match (deletePreconditionOK).
- **RR-3FK40L (minor)** — cursor doc claimed opaque but exposed raw seq; 422-detail leak inconsistency. Doc fixed; 422 detail confirmed metamodel-only.

Reviewer confirmed correct: writeMu TOCTOU closed, 412-before-422 precedence,
path-traversal allowlist, syncContext identity-neutral.

## Seam note

manifest/applier are derived lazily (syncManifest()/syncApplierFor() type-assert
a.store / a.entityManager per request) rather than cached in NewApp — so they
stay correct when the test harness re-points the store/manager after
construction (rebindApp), which production never does.
