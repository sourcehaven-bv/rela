---
id: PLAN-KZ5H
type: planning-checklist
title: 'Planning: Theme packages: export and install bundled colors, font, and logo in data-entry'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**

- New `.relatheme` zip package format containing manifest + optional logo + optional font.
- Backend persistence under `.rela/theme/` (via existing `kv` abstraction, like `.rela/palette.yaml`).
- Backend HTTP routes for theme export, install, and serving the assets to the SPA.
- Settings UI additions: Logo upload + preview, Font upload + preview, Export button, Install button.
- Apply the bundled font as the UI font via a CSS `@font-face` declaration loaded from the asset URL.
- Apply the bundled logo as the sidebar branding image (replacing/supplementing the current text-only `appName`).
- Reuse the existing palette validation, save flow, and `kv` storage — no new abstractions.
- Install flow: matches the existing palette flow — populates the editor; user clicks the existing Save buttons to apply.

**Out of scope:**

- Multi-theme libraries / switching between several saved themes (one active theme only).
- Project-level theme defaults baked into the binary (project ships `data-entry.yaml` palette already; this ticket is purely user-side).
- Web fonts loaded over HTTP (only locally-bundled font files).
- Per-component icon packs, animations, sound packs, etc.
- Cryptographic signing or trust verification for theme packages.
- Drag-and-drop install (use a standard `<input type=file>`).
- Theme registry / discoverability (themes are just files).
- Multi-resolution raster logo bundles (`srcset` / `<picture>` / `@2x`
  variants). Vector marks are covered by SVG, which is in the allowlist;
  raster bundles would require a richer `logo` manifest object and additional
  asset slots. The manifest can grow `logo: string` → `logo: object` later
  without breaking existing themes.

**Acceptance Criteria:**

1. **Settings page exposes Logo upload, Font upload, and Theme Export buttons.** Test: open Settings → Appearance, verify three new affordances are visible alongside existing palette controls.
2. **Theme Export produces a `.relatheme` zip downloadable by the browser.** Test: with palette + logo + font set, click Export; verify a `.relatheme` file downloads and unzipping reveals `theme.yaml`, `logo.<ext>`, `font.<ext>`.
3. **Theme Install accepts a `.relatheme` zip and stages colors / logo / font into the editor.** Test: click Install, choose a `.relatheme` file; verify color inputs populate, logo preview updates, font preview updates. The user then clicks Save palette / Save logo / Save font (existing pattern) to persist.
4. **Bundled fonts load via @font-face and apply to data-entry text.** Test: install a theme with a recognizable font; after Save, the sidebar / form labels render in that font.
5. **Bundled logos replace the existing branding text in the sidebar.** Test: install a theme with a logo; after Save, the sidebar shows an `<img>` instead of the text `appName`. When no logo is set, sidebar falls back to text (current behavior).
6. **Invalid / corrupt theme files surface a clear error to the user; existing theme is preserved.** Test cases: not-a-zip, zip without `theme.yaml`, manifest with invalid YAML, manifest with bad hex colors, font file too large, image file too large, font with disallowed mime, image with disallowed mime → each produces a clear toast error and leaves all current settings unchanged.
7. **Theme persistence reuses the existing settings system.** Implementation review: changes only extend `palette.yaml` (or co-located files in `.rela/theme/`); no new repo / transaction / store abstractions introduced.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Zip archive in Go:** stdlib `archive/zip` — chosen, no third-party dep needed. Read via `zip.NewReader(bytes.NewReader, size)`; write via `zip.NewWriter(w)`.
- **Existing palette plumbing (`internal/dataentryconfig/palette.go`):** `PaletteConfig`, `ValidatePalette`, `ResolvePalette` — the theme manifest will reuse `PaletteConfig` directly so light/dark/badge logic is unchanged.
- **Existing `kv` abstraction in `internal/dataentry/app.go`:** already used for `user-defaults.yaml`, `palette.yaml`, `ui-state.json`. Reuse for `theme/manifest.yaml`, `theme/logo.<ext>`, `theme/font.<ext>` (separate files inside a sub-key, since the manifest lives next to existing files).
- **Existing palette save flow (`SettingsView.handleSavePalette`):** Edit → Save button → POST → schemaStore.reload(). Theme install will follow this — populate editor, user clicks existing Save buttons.
- **Reference: VS Code theme zips (.vsix), Obsidian theme dirs:** both decoupled palette + (optional) assets via manifest. We mirror the manifest pattern with a single `theme.yaml`.
- **Reference: `data-entry.yaml` `app.name` (handled in `schemaStore.app.name`, used by `Sidebar.vue:25`):** there is no existing image-based logo pipeline in the SPA. The Sidebar renders `appName` as text only. This ticket adds an optional image to the same slot.
- **Existing dependencies** (`frontend/package.json`): no zip library yet client-side. Use the **JSZip** library (small, MIT) — it is the de-facto standard. Alternative: hand-roll with the browser-native `CompressionStream` (only zlib, not zip container). Verdict: JSZip.

**Files to modify (preliminary, may grow during implementation):**

Backend (Go):

- `internal/dataentryconfig/palette.go` (or new `theme.go` next to it): add `ThemeManifest` type with embedded `PaletteConfig` + optional `Logo` + `Font` filename pointers. Add `ValidateThemeManifest`.
- `internal/dataentry/app.go`: add `loadUserTheme` / `saveUserTheme` (manifest), `loadUserLogo` / `saveUserLogo`, `loadUserFont` / `saveUserFont`. Add `UserTheme`, `UserLogoFilename`, `UserFontFilename` fields to `AppState`.
- `internal/dataentry/handlers_api.go`:
  - `GET /api/v1/_theme/logo` → serve bytes (with proper Content-Type, ETag, Cache-Control: no-store while editing).
  - `GET /api/v1/_theme/font` → serve bytes.
  - `PUT /api/v1/_theme/logo` (multipart) → save logo.
  - `PUT /api/v1/_theme/font` (multipart) → save font.
  - `DELETE /api/v1/_theme/logo` → remove logo.
  - `DELETE /api/v1/_theme/font` → remove font.
  - `GET /api/v1/_theme/export` → produces a `.relatheme` zip with the current user palette + logo + font.
  - `POST /api/v1/_theme/import` (multipart, `.relatheme`) → unpacks; returns the parsed `PaletteConfig` + the saved logo/font URLs (does NOT persist palette, only stages logo/font and returns palette JSON for the editor).
- `internal/dataentry/api_v1.go`: register the new routes.

Frontend (Vue/TS):

- `frontend/package.json`: add `jszip` runtime dep.
- `frontend/src/api/theme.ts` (new): typed clients for `getLogoUrl()`, `uploadLogo(file)`, `uploadFont(file)`, `deleteLogo()`, `deleteFont()`, `exportTheme()`, `importTheme(file): Promise<{palette, logoUrl?, fontUrl?}>`.
- `frontend/src/views/SettingsView.vue`: add three new sub-sections in Appearance: Logo (upload + remove), Font (upload + remove), Theme package (Export, Install).
- `frontend/src/components/common/Sidebar.vue`: render `<img class="logo-img" :src=logoUrl />` when a logo is set; fall back to text otherwise.
- `frontend/src/stores/ui.ts` or `frontend/src/stores/schema.ts`: track `logoUrl`, `fontFamily` reactively; `applyFont(url)` injects a `@font-face` rule and sets `--ui-font` CSS var on `:root`.
- `frontend/src/App.vue`: declare `--ui-font` CSS variable and use it in the global `font-family` rule (replacing/wrapping the existing system-font stack so we keep the fallback chain).
- `frontend/src/utils/theme-package.ts` (new): pure helpers to build/parse the zip in-browser using JSZip. Used for client-side export and to *show* the manifest contents during install before round-tripping to the server.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Theme package format (`.relatheme`)

A standard zip archive with these entries:

```text
theme.yaml          # required: ThemeManifest (YAML)
logo.<ext>          # optional: PNG / JPEG / SVG / WebP
font.<ext>          # optional: WOFF2 / WOFF / TTF / OTF
```

`theme.yaml` shape (a superset of `palette.yaml`, so existing palettes are valid
manifests with name/version added):

```yaml
name: "My Theme"            # required, 1-100 chars
version: "1.0.0"            # required, semver-ish (any non-empty string for now)
author: "..."               # optional
# colors: identical shape to existing palette.yaml
base: "#1a1a2e"
surface: "#f8fafc"
accent: "#6366f1"
# ... (all 8 role keys, badges, dark — same as PaletteConfig)
logo: "logo.png"            # optional, references zip entry
font:                       # optional
  filename: "font.woff2"    # references zip entry
  family: "MyFont"          # required if font block present (used in @font-face)
```

### Backend export flow

1. `GET /api/v1/_theme/export` reads the current user palette, logo bytes, font bytes from `kv`.
2. Composes a `ThemeManifest` (palette fields + name defaulting to `app.name`, version `1.0.0`, logo/font entries if present).
3. Writes a zip to `bytes.Buffer` using `archive/zip`; returns it with `Content-Type: application/zip` and `Content-Disposition: attachment; filename="<safe-name>.relatheme"`.

### Backend install flow

1. `POST /api/v1/_theme/import` accepts `multipart/form-data` with a single file (`.relatheme`).
2. Validates: zip parses; `theme.yaml` exists, parses, validates as `ThemeManifest`; logo/font entries (if referenced) exist in zip and pass mime-type allowlist + size limit (logo ≤ 256 KiB, font ≤ 2 MiB).
3. **Persists logo and font bytes** to `.rela/theme/logo.<ext>`, `.rela/theme/font.<ext>` via `kv`. (Reasoning: assets need a stable URL the browser can fetch; staging only in memory wouldn't survive page reload.)
4. **Does NOT persist the palette.** Returns the parsed palette JSON in the response so the SPA stages it in the existing palette editor; user clicks "Save palette" to commit, matching the current pattern.

Trade-off: this means logo/font are committed atomically on install, but palette
is not. That mismatch is acceptable because logo/font are bytes (you can't
reasonably "stage" them without a temp store) and the user can still un-install
via the Remove button. Alternative considered: tempfile staging with a "commit"
endpoint — rejected as overengineered for an MVP.

### Frontend install UX

- Click "Install theme" → file picker (`accept=".relatheme,application/zip"`).
- POST file to `/api/v1/_theme/import`. Backend persists logo/font and returns the palette.
- On response, call existing palette load functions to populate the editor with the returned palette (just like the current "Reset palette" path), then call `schemaStore.reload()` to pick up the new logo/font URLs.
- Toast: "Theme installed. Click Save to persist colors."

### Logo wiring in Sidebar

- `schemaStore` exposes `logoUrl` (computed): `userTheme?.logo ? '/api/v1/_theme/logo?v=<hash>' : null`.
- `Sidebar.vue`: `<img v-if="logoUrl" :src="logoUrl" />` else `{{ appName }}`. Cache-bust query param uses content hash so updates are immediate.

### Font wiring

- On schemaStore load (or after install), if `userTheme?.font`, inject a `<style>` in `<head>`: `@font-face { font-family: "<family>"; src: url("/api/v1/_theme/font?v=<hash>") }` and set `:root { --ui-font: "<family>", -apple-system, ... }`.
- `App.vue` global `font-family` rule already uses a system stack — change it to `var(--ui-font, -apple-system, BlinkMacSystemFont, ...)`.

### Persistence layout

```text
.rela/
  palette.yaml         # existing, unchanged
  user-defaults.yaml   # existing, unchanged
  theme/
    manifest.yaml      # name, version, author, font.family — metadata only
    logo.<ext>         # bytes
    font.<ext>         # bytes
```

The manifest under `theme/` carries metadata (name / version / author / font
family) that isn't part of `palette.yaml`. We deliberately keep the palette in
its existing `palette.yaml` location rather than denormalising it into the theme
manifest, so users who only edit colors don't need to know about themes.

**Alternatives considered:**

1. **Embed assets as base64 in `palette.yaml`.** Rejected: bloats the YAML, awkward for binary, and violates the "permissive markdown + YAML" philosophy in CLAUDE.md.
2. **Browser-only theme storage (localStorage).** Rejected: doesn't match the existing settings system; loses the theme on cache clear; user picked the disk-backed option.
3. **Apply theme immediately on install (no Save step).** Rejected per user clarification — the current palette flow requires explicit Save, themes should match.
4. **Use only browser-native `CompressionStream` (no JSZip).** Rejected: only handles zlib, not zip container format.
5. **Sign theme packages.** Rejected: out of scope; no trust model in rela today.

**Files to modify:** see Research section above.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
|---|---|---|
| `.relatheme` upload | Multipart from logged-in user | Size cap on entire upload (≤ 5 MiB). Must parse as zip. Must contain `theme.yaml`. |
| `theme.yaml` content | Inside zip | YAML parse + `ValidateThemeManifest` (allowlist of keys; reuse `ValidatePalette` for color fields; require name 1–100 chars; font.family 1–64 chars matching `[A-Za-z0-9 _-]`). |
| Logo bytes | Inside zip | Mime-sniff allowlist: `image/png`, `image/jpeg`, `image/svg+xml`, `image/webp`. Size ≤ 256 KiB. **SVG: also strip `<script>`, `on*=` attributes, and `xlink:href` to non-data URIs** (or reject SVG entirely if sanitization is judged risky in review — both options on the table). |
| Font bytes | Inside zip | Magic-byte check for WOFF2 (`wOF2`), WOFF (`wOFF`), TTF (`\x00\x01\x00\x00` / `OTTO`), OTF. Size ≤ 2 MiB. |
| Logo upload (separate) | Multipart | Same allowlist + size as above. |
| Font upload (separate) | Multipart | Same allowlist + size as above. |

**Security-Sensitive Operations:**

- **Zip path traversal:** parse zip entries by name only (`logo.<ext>`, `font.<ext>`); never use `entry.Name` to construct a filesystem path. Reject any entry whose normalized name contains `/`, `\`, or `..`.
- **Zip-bomb protection:** uncompressed-size cap per entry + total cap. Reject zip with declared/observed expansion ratio > 100×.
- **Stored XSS via SVG logo:** the `<img src>` attribute renders SVGs as images, which DOES NOT execute scripts (browsers sandbox `<img>`-loaded SVGs). Confirm we never inline the SVG. For belt-and-braces, serve with `Content-Security-Policy: sandbox`.
- **Stored XSS via filename:** never reflect the uploaded filename into HTML; we always rename to `logo.<ext>` / `font.<ext>` based on sniffed type.
- **Authn:** these endpoints inherit the existing data-entry auth (none today; localhost-only). No new attack surface beyond what `_palette` already has.
- **Error messages:** report failure category (`"unsupported font format"`, `"manifest missing 'name' field"`) without echoing back input bytes, file headers, or stack traces.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | How tested |
|---|---|
| 1. Settings exposes Logo / Font / Export buttons | `SettingsView` mount test (Vitest) — verify the three new controls are present. |
| 2. Export produces valid `.relatheme` | Go test for `/api/v1/_theme/export`: set up app with palette + logo + font, hit endpoint, parse response as zip, assert `theme.yaml` parses and `logo.png` / `font.woff2` are present with the expected bytes. |
| 3. Install populates editor + applies after Save | E2E (Playwright in `/e2e/`): export from one app, install into a fresh app, verify editor populated; click Save; reload; verify persisted. |
| 4. Font applied via @font-face | E2E: install theme with `Lobster.woff2`; assert `getComputedStyle(sidebar).fontFamily` includes `Lobster`. |
| 5. Logo replaces sidebar text | E2E: install theme with logo; assert `<img>` present in sidebar header; remove logo; assert text fallback. |
| 6. Invalid theme files | Go table-driven test for `/api/v1/_theme/import`: each row is a malformed zip, assert 400 + specific error code. Vitest test for client-side rejection (file > 5MB before upload). |
| 7. No new abstractions | Code review checks; `just arch-lint` must pass. |

**Edge Cases:**

- Manifest with palette ONLY (no logo, no font) — valid; round-trips through export/import.
- Manifest with logo only / font only.
- Light-only palette vs light+dark palette in the manifest.
- Zip with extra unrelated entries (`README.md`, `.DS_Store`) — ignore unknown entries with no error.
- Logo file referenced in manifest but missing in zip → reject manifest at install.
- Empty zip → reject.
- Non-UTF8 bytes in `theme.yaml` → YAML parser surfaces error.
- Extension mismatch (logo says `logo.png`, bytes are JPEG) — accept on sniffed type, normalize stored filename to sniffed extension.
- Unicode characters in name / family.
- Concurrent imports from two browser tabs — last write wins (matches existing `palette.yaml` behavior; `mutateState` serializes).
- Reload while font is loading — `<style>` re-injection is idempotent.
- Removing a logo when none is set → 204, no error.

**Negative Tests:**

- Upload non-zip → 400 "not a valid zip".
- Upload zip without `theme.yaml` → 400 "missing theme.yaml".
- `theme.yaml` with malformed YAML → 400 with parse error category.
- `theme.yaml` with bad hex color → 400 "invalid color".
- Logo file > 256 KiB → 400 "logo too large".
- Font file > 2 MiB → 400 "font too large".
- Total zip size > 5 MiB → 400.
- Logo with disallowed mime (e.g. `application/octet-stream`) → 400.
- Font that fails magic-byte check → 400.
- Zip-bomb (entry with 100× expansion) → 400 "compressed entry too large".
- Path-traversal entry name (`../foo`) → 400 "invalid zip entry".

**Integration test approach:**

- New Go integration test in `internal/dataentry/handlers_api_test.go` that drives export then import end-to-end against an in-memory `kv`, asserting that the round-trip preserves palette + logo bytes + font bytes byte-for-byte.
- Playwright e2e in `/e2e/` that exercises the full UI flow.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| SVG XSS via logo | Medium | High | `<img>`-only rendering (no inline SVG); CSP sandbox header; test with known-malicious SVG fixture. If sanitization adds significant complexity, drop SVG from the allowlist for v1 and add it later. |
| Font file licensing user error | High | Low | Out of our control (user's responsibility); add a one-line warning under the font upload control: "Only upload fonts you are licensed to redistribute". |
| Logo / font bloats `.rela/` directory in git | Medium | Low | Document that `.rela/theme/` should be gitignored (it follows the same convention as `.rela/user-defaults.yaml` per CLAUDE.md). |
| Browser caches old logo after replacement | High | Low | Cache-bust query param using a content hash. |
| Zip parsing CPU on large file | Low | Medium | 5 MiB upper bound; `archive/zip` reader is O(n) with no decompression on file listing. |
| `appName` text already accommodates long strings; image will require new CSS | Low | Low | Add `.logo-img { max-height: ...; max-width: 100%; object-fit: contain; }` and visually verify. |

**Effort:** **m** (already set on the ticket). Backend: ~4 new endpoints +
manifest type + tests. Frontend: ~3 UI controls + ~2 stores updates + JSZip
integration + tests + e2e. Realistic estimate 1.5–2 working days.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — add a "Theme packages" section to the data-entry user docs explaining export/import format.
- [ ] CLI help text — N/A (no CLI surface).
- [x] CLAUDE.md — add a brief note in the "Don't do this" list reminding future contributors not to embed binary assets in YAML.
- [ ] README.md — N/A.
- [x] API docs — document the new `/api/v1/_theme/*` endpoints alongside the existing `_palette` docs.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
