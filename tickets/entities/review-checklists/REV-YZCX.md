---
id: REV-YZCX
type: review-checklist
title: 'Review: Custom logo upload for data-entry sidebar branding'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — all packages pass; 75.4 % total coverage.
- [x] Lint clean (`just lint`) — 0 issues.
- [x] Coverage maintained (`just coverage-check`) — package floor satisfied; total 75.4 % > 65 % threshold.
- [x] Architecture lint clean (`just arch-lint`).
- [x] Frontend tests pass (`npm run test:run`) — 655/655.
- [x] Frontend typecheck clean (`npm run typecheck`).
- [x] Frontend lint clean (`npm run lint`) — 0 errors.

## Code Review

- [x] Run `cranky-code-reviewer` agent — done; 10 findings.
- [x] All critical review-responses addressed — none filed.
- [x] All significant review-responses addressed — RR-290V, RR-DIL7, RR-4OMU, RR-OXZF.
- [x] Self-reviewed the diff for unrelated changes.

**Review Responses:**

| ID | Severity | Status |
|---|---|---|
| RR-290V | significant | addressed (CSP `frame-ancestors 'none'` + `X-Frame-Options: DENY`) |
| RR-DIL7 | significant | addressed (looksLikeSVG hardened; 9 polyglot test cases) |
| RR-4OMU | significant | addressed (envelope headroom 4 KiB → 16 KiB) |
| RR-OXZF | significant | addressed (`AppState.LogoURL()` factored out) |
| RR-KOIB | minor | addressed (named results dropped + `nolint:gocritic`) |
| RR-NSKU | nit | addressed (collision-comment rewritten) |
| RR-YBW8 | minor | addressed (server `maxBytes` in 413; client uses it) |
| RR-LUEU | nit | addressed (mutateState republish comment) |
| RR-8B67 | minor | addressed (TestThemeLogo_SVGGetHeaders) |
| RR-LQXY | nit | addressed (loadUserLogo concurrency comment) |

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-HK8S)

**Acceptance Status:**

| AC | Result | Evidence |
|---|---|---|
| 1. Settings shows Logo control | PASS | `frontend/src/views/SettingsView.vue` adds `<section>` with file picker, preview, Upload + Remove buttons. |
| 2. Upload persists + sidebar replaces text | PASS | curl `PUT /api/v1/_theme/logo` → 200 `{logoUrl: ".../logo?v=<hash>"}`; `_sidebar` exposes the same hash; `Sidebar.vue` renders `<img>` when set. |
| 3. Remove → text fallback | PASS | `DELETE` → 204; `_sidebar.logoUrl` becomes nil; `<span>{{ appName }}</span>` rendered. |
| 4. Validation matrix | PASS | `TestThemeLogo_Validation` (GIF, plain text, missing field, empty); `TestThemeLogo_TooLarge`, `TestThemeLogo_ExactlyAtLimit`. |
| 5. SVG `<script>` neutralized | PASS | `TestThemeLogo_SVGGetHeaders` asserts `Content-Type: image/svg+xml`, `nosniff`, `CSP: sandbox; frame-ancestors 'none'`, `X-Frame-Options: DENY`. Browser sandbox handles the runtime side. |
| 6. Sidebar layout | PASS | `.logo-img { max-height: 28px; max-width: 100%; object-fit: contain }` + image inherits existing `.collapsed .logo { display:none }`. |

## Documentation (enhancements only)

Deferred to PR 3 (TKT-WPKW). The umbrella plan called for documenting the logo
control alongside the broader theme system; with PR 1 merging independently,
end-user docs land with the packaging story. The API endpoints are
self-documented via the test suite + this checklist.

- [x] Logged in `IMPL-HK8S` Documentation Planning section.

## Final Checks

- [x] Commit message will explain why (theme-system PR 1, depends-on TKT-WPKW).
- [x] No TODOs or FIXMEs left unaddressed.
- [x] Ready for another developer to use.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- to be added by /pr -->
