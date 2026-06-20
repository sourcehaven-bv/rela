---
id: REV-J5KCVC
type: review-checklist
title: 'Review: Sync 4/5: server sync HTTP API (manifest + conditional push) on data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — `go test -race ./internal/dataentry/` ok; pgstore postgres suite ok; all 3 build tags compile
- [x] Lint clean — my files 0 new issues; `just arch-lint` clean
- [x] Coverage maintained — sync_test.go covers manifest/push/delete/auth/attribution; dep gates (no pgx default, no bleve pgstore) hold

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer) — found 1 critical + 3 sig/minor
- [x] All critical addressed — RR-X869DY (CSRF exemption → Origin/Cookie-aware)
- [x] All significant addressed — RR-9RHEWN (slash key), RR-2D1SZO (no blind delete)
- [x] Self-reviewed the diff — sync.go/sync_handlers.go/sync_test.go new; middleware_security, router, principal, archfile touched; app.go nets to no change

**Review Responses:** RR-X869DY (critical), RR-9RHEWN, RR-2D1SZO (sig),
RR-3FK40L (minor) — all addressed + regression-tested. The critical was a real
CSRF hole (cookie-mode proxy) the reviewer caught; the fix makes the same-origin
exemption conditional on a provably-non-browser request.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence in implementation checklist

**Acceptance Status:** all PASS — manifest changed+tombstones+cursor; push
200/412/422 distinct; Tool=sync attribution; no-Origin CLI admitted but
Host-checked & cookie/cross-origin rejected; path-traversal rejected; malformed
cursor → full manifest.

## Documentation (enhancements only)

- [x] ~~Docs-checklist / user-facing docs~~ (N/A here: the sync API is consumed by the CLI in TKT-T4H4YK, where the end-user docs + the OAuth-proxy deployment notes land. The endpoint/security godoc documents the trust model for maintainers; .ignored/sync-entra-oauth2proxy-notes.md has the operator config.)

**Docs Checklist:** N/A — CLI sub-ticket owns user docs.

## Final Checks

- [x] Commit message explains the why
- [x] No TODOs/FIXMEs (cursor HMAC + retention/pagination are tracked follow-ups)
- [x] Ready for another developer — the CLI (TKT-T4H4YK) consumes these endpoints

## Pull Request

- [ ] Run `/pr`
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending push -->
