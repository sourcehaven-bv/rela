---
id: IMPL-VIQW
type: implementation-checklist
title: 'Implementation: Theme packages: export/install bundled palette + logo as .relatheme zip'
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
| `internal/dataentryconfig/theme.go` | `theme_test.go`: accept matrix, reject matrix, YAML round-trip with key-collision sentinel. |
| `internal/dataentry/theme_package.go` | `theme_package_test.go`: palette-only / with-logo / sniff-trumps-manifest-extension / extras-ignored / reject matrix (missing manifest, name empty, missing logo, bytes-not-image, path traversal, subdirectory entries, not-a-zip, malformed yaml, bad palette color) / logo-too-large / total-too-large / zip-bomb. |
| `internal/dataentry/handlers_theme_package.go` | `handlers_theme_package_test.go`: export with/without logo, full round-trip (export → import → state assertions), error paths (not-zip, missing field), method-not-allowed for both endpoints, `safeThemeFilename` table. |
| `frontend/src/api/theme.ts` (new exports) | `frontend/src/api/theme.test.ts`: exportTheme triggers download via `URL.createObjectURL`, importTheme POSTs FormData with `file` field, both surface server errors. |

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`buildZip(t, entries)` and `minimalManifestYAML(extra)` are test helpers so each
test only specifies the bytes/fields that matter to its scenario. Round-trip
test compares against the original input bytes (`bytes.Equal(dest.UserLogoBytes,
pngBytes)`), not hardcoded literals. Reject matrix uses sentinel-error matching
(`errors.Is`) rather than substring comparisons.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built `cmd/rela-server` against the SPA bundle, ran against `tickets/`,
exercised the round-trip via curl:

| AC | Manual check |
|---|---|
| 1. Settings shows Export/Install | Theme Package card rendered between Logo and App Info; Export + Install buttons visible. |
| 2. Export with palette + logo | `GET /api/v1/_theme/export` returned `application/zip` (394 bytes) with `Content-Disposition: attachment; filename="..."`; `unzip -l` showed `theme.yaml` (99 bytes) + `logo.png` (56 bytes). |
| 3. Export with no logo | Confirmed by Go test `TestThemeExport_PaletteOnly`. Manifest carries no `logo:` field; zip has only `theme.yaml`. |
| 4. Import staging | `POST /api/v1/_theme/import` returned `{logoUrl: ".../logo?v=<hash>", palette: {accent: "#aabbcc", badges: {...}}}`. The existing `/_palette` endpoint still showed the previous saved palette afterward — confirming palette is *staged*, not auto-saved. |
| 5. Live sidebar update | After import, `_sidebar.logoUrl` reflected the new hash. |
| 6. Validation matrix | Curl tests: non-zip → 400 `not a valid zip file`; zip with subdirectory entry → 400 `zip entry contains path traversal`. |
| 7. No new abstractions | `just arch-lint`: clean. Backend uses `state.KV` + `mutateState` + reused `saveUserLogo` / `sniffLogoMime`. No repo / transaction layers. |

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

- Mirrors the merged logo PR's plumbing: state-cached snapshot via `mutateState`, structured 413 with `maxBytes`, reuse of `sniffLogoMime` / `MaxUserLogoBytes` / `saveUserLogo` / `logoURLForHash`.
- Stdlib zip only — no third-party Go dep.
- Server-side parsing only — `parseThemePackage` is a pure helper, no `App` access; trivially unit-testable.
- Layered defenses: total-size cap, expansion-ratio cap, path-traversal check, manifest validation, sniff-on-bytes for logos.
- Lint clean (`just lint`); coverage threshold satisfied (75.5 % total).
- All Go tests pass (`just test`); all 659 frontend tests pass (`npm run test:run`).
