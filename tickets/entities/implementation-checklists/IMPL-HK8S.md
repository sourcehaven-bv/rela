---
id: IMPL-HK8S
type: implementation-checklist
title: 'Implementation: Custom logo upload for data-entry sidebar branding'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**New code under test:**

| File | Test |
|---|---|
| `internal/state/state.go` (`Delete`) | `internal/state/state_test.go`: `TestFSKV_Delete_RemovesKey`, `TestFSKV_Delete_MissingKeyIsNotError`, `TestFSKV_Delete_RejectsInvalidKey` (state package coverage 100 %). |
| `internal/dataentry/theme_logo.go` | Exercised via the handler tests (round-trip writes both files, then reads them back; deletion verifies cleanup). |
| `internal/dataentry/handlers_theme.go` | `internal/dataentry/handlers_theme_test.go`: round-trip, validation matrix (GIF / plain / wrong field / empty), accepted formats (PNG / JPEG / WebP / SVG), too-large, exactly-at-limit, idempotent delete, GET-when-unset, method-not-allowed, settings-exposes-logoUrl. |
| `frontend/src/api/theme.ts` | `frontend/src/api/theme.test.ts`: PUT shape (FormData with `logo` field), error propagation, DELETE round-trip. |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Test fixtures use builder helpers (`makeTinyPNG`, `jpegMagicBytes`,
`gifMagicBytes`, `webpMagicBytes`, `hostileSVGBytes`, `uploadLogo`) so each test
only specifies the bytes that matter to it. Assertions compare against the
original input bytes (`bytes.Equal(s.UserLogoBytes, pngBytes)`), not hardcoded
literals.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `cmd/rela-server` with the new SPA bundle and ran it against `tickets/`.
Verified each AC via `curl`:

| AC | Manual check |
|---|---|
| 1. Settings shows Logo control | Vitest mount tests for `theme.test.ts` cover the API path; `SettingsView.vue` template integration verified by visual inspection of the dev build (Choose / Upload / Remove buttons appear in the new Logo card with the preview frame). |
| 2. Upload persists + immediate sidebar replacement | `PUT /api/v1/_theme/logo` with a 56-byte PNG returned `{ok:true, logoUrl:"/api/v1/_theme/logo?v=9bd6c68d0d8f"}`. Subsequent `GET /api/v1/_sidebar` exposed the same `logoUrl` so the SPA renders `<img>` instead of text. |
| 3. Remove → text fallback | `DELETE` returned 204; subsequent `GET /api/v1/_theme/logo` returned 404 + `{error:"no logo set"}`; `_sidebar` no longer included `logoUrl`. |
| 4. Mime/size validation | `PUT` with `GIF89a` body returned `400 {error:"unsupported format: image/gif (accepted: image/png, image/jpeg, image/svg+xml, image/webp)"}`. Plain text + missing-field paths covered by Go tests. |
| 5. SVG `<script>` neutralized | Hostile-SVG fixture (`<script>`, `xlink:href` to invalid host, `onload`) accepted at upload (correctly identified as `image/svg+xml`). Response carries `Content-Type: image/svg+xml`, `X-Content-Type-Options: nosniff`, `Content-Security-Policy: sandbox` — confirmed via `curl -i`. The hostile patterns are inert when rendered via `<img>` (browser sandbox; the unit test verifies the response headers; full e2e is the Playwright job once we have one). |
| 6. Sidebar layout | Inspected dev build: `.logo-img { max-height: 28px; max-width: 100%; object-fit: contain }` keeps the image inside the header. The image inherits `.collapsed .logo { display:none }` since it's wrapped by the existing `.logo` link. |

**Persistence across restart:** uploaded an SVG, killed the server (`killall
rela-server`), restarted it. `_sidebar` still reported the same `logoUrl`
(`?v=f04fb6cf03cc`) and `GET /api/v1/_theme/logo` served the correct bytes with
`Content-Type: image/svg+xml`. Disk layout matched the plan: `.rela/theme/logo`
(267 bytes) + `.rela/theme/logo.ext` (3 bytes, content `svg`).

**Headers verified on the live response:**

```
HTTP/1.1 200 OK
Cache-Control: public, max-age=86400, immutable
Content-Security-Policy: sandbox
Content-Type: image/png
X-Content-Type-Options: nosniff
```

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

- Mirrors the existing `_palette` plumbing (state-cached snapshot, `mutateState` for atomic publish, error-on-corrupt-load).
- New file boundary respects `arch-lint` (passes).
- Lint clean (`just lint`); coverage threshold satisfied (`just coverage-check` shows 75.4 % total, 100 % `state`).
- All Go tests pass (`just test`); all 655 frontend tests pass (`npm run test:run`).
