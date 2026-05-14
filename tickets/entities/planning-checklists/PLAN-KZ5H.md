---
id: PLAN-KZ5H
type: planning-checklist
title: 'Planning: Theme packages: export and install bundled palette + logo as .relatheme zip'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope (TKT-WPKW, builds on the merged logo PR TKT-WN7O):**

- New `.relatheme` zip format containing a YAML manifest plus an optional logo image.
- Backend endpoints under `/api/v1/_theme`:
  - `GET /api/v1/_theme/export` → returns a `.relatheme` zip download.
  - `POST /api/v1/_theme/import` → accepts a `.relatheme` upload, persists the logo (if present), and returns the parsed palette JSON for the editor (does NOT auto-save the palette).
- Manifest type: `ThemeManifest` defined alongside `dataentryconfig.PaletteConfig` (palette fields embedded; adds `name`, `version`, `author`, optional `logo` filename reference).
- Frontend: Settings → Appearance gains a Theme package sub-section with **Export** and **Install** buttons.
- Install flow: persists the logo bytes immediately (matches PR 1's PUT behaviour), stages the palette into the existing palette editor for an explicit Save.
- Standard library zip (`archive/zip`) — no new third-party Go dependency.
- No browser-side zip parsing — server handles both zip directions, frontend uses `fetch` + `Blob`.

**Out of scope (deferred):**

- **Custom UI fonts** — the font slot is omitted from the manifest entirely. Adding it later is non-breaking (manifest can grow).
- **Custom CSS overrides** — explicit non-goal; large attack surface, no concrete user need.
- **Multi-resolution rasters** (`srcset` / `<picture>` / `@2x`).
- **Multi-theme libraries** / switching between several saved themes.
- **Theme registries / discoverability** — themes are just files.
- **Cryptographic signing or trust verification** for theme packages.
- **Drag-and-drop install** — standard `<input type="file">` only.
- **Project-level theme defaults baked into the binary** — already covered by `data-entry.yaml`.

**Acceptance Criteria:**

1. **Settings page exposes Export and Install buttons in the Appearance section.** Test: Vitest mount of `SettingsView` asserts the Theme package sub-section has both buttons.
2. **Theme Export produces a `.relatheme` zip containing `theme.yaml` plus `logo.<ext>` when set.** Test (Go): set up an app with a palette + logo, hit `/api/v1/_theme/export`, parse response as zip, assert `theme.yaml` parses cleanly and `logo.<ext>` is present with byte-identical content.
3. **Theme Export with no logo set produces a zip containing only `theme.yaml`.** Test (Go): same as above with logo unset; assert `theme.yaml` exists and there is no `logo.*` entry.
4. **Theme Install accepts a `.relatheme` zip, stages palette into the editor.** Test (e2e Playwright): export from one app instance, install into a fresh app instance, verify color inputs in the editor match the source. Click Save palette; verify persistence.
5. **Theme Install persists the bundled logo immediately and the sidebar updates without reload.** Test (e2e): install a theme containing a logo, observe `<img>` in sidebar without a page reload (uses the live-update mechanism added during PR 1's review fixup).
6. **Invalid / corrupt theme files surface a clear toast error and leave existing settings unchanged.** Test cases (Go table): not-a-zip, zip without `theme.yaml`, manifest with malformed YAML, manifest with bad hex colors, logo entry referenced but missing, oversized logo, total zip > 5 MiB. Each row asserts the response code and that `AppState.UserPalette` / `AppState.UserLogoBytes` are unchanged.
7. **Theme persistence reuses existing storage patterns.** Implementation review + `just arch-lint`: no new repo / transaction abstractions; palette stays in `palette.yaml`; logo stays at `theme/logo` + sidecar.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Stdlib `archive/zip`** — fully sufficient. Read via `zip.NewReader(bytes.NewReader, size)`; write via `zip.NewWriter(w)`. No third-party dep.
- **Existing palette plumbing** (`internal/dataentryconfig/palette.go`):
  - `PaletteConfig` (line 24) — exported struct already used by the `_palette` API.
  - `ValidatePalette(p *PaletteConfig) error` (line 210) — central validator; we reuse it on the imported manifest's color fields.
  - `ResolvePalette(project, user *PaletteConfig) *ResolvedPalette` (line 295) — used at app boot. Not needed by import (we don't auto-save palette).
- **Logo plumbing** (merged via TKT-WN7O):
  - `internal/dataentry/theme_logo.go`:
    - `userLogoFile` (`"theme/logo"`) and `userLogoExtFile` (sidecar) — same storage we re-use for imports.
    - `MaxUserLogoBytes` (256 KiB), `allowedLogoExts`, `logoExtForMime`, `logoContentType` — all reusable.
    - `loadUserLogo`, `saveUserLogo`, `deleteUserLogo`, `hashLogoBytes`, `(*AppState).LogoURL()` — ready to call.
  - `internal/dataentry/handlers_theme.go`:
    - `sniffLogoMime`, `looksLikeSVG`, `writeLogoTooLarge`, `logoURLForHash` — reuse where they fit; do not extend gratuitously.
- **Existing settings save flow** (`SettingsView.handleSavePalette`, `handleAPISavePalette`): this is the pattern install must blend into. Stage the palette via `loadPaletteState` / setting the editor refs; user clicks the existing **Save Palette** button to commit.
- **Existing live logo update** (added during PR 1 review): `schemaStore.setLogoUrl(url)` causes the Sidebar to swap to the new image without a page reload. Install pushes the URL through this same setter.
- **Reference packagings:** VS Code `.vsix` (zip + JSON manifest), Obsidian theme dirs, Sketch `.sketchpalette`. The pattern of "manifest + asset files inside a zip" is standard; we mirror it.
- **No third-party deps needed.** JSZip was considered when I thought we'd parse zips client-side; rejected — server-side parsing is simpler and reuses the existing trust boundary.

**Files to modify:**

Backend (Go):

- `internal/dataentryconfig/theme.go` (new): defines `ThemeManifest`. Embeds `PaletteConfig`. Adds:
  - `Name string` (required, 1–100 chars).
  - `Version string` (required, any non-empty 1–32 char string for v1 — no semver enforcement).
  - `Author string` (optional, ≤100 chars).
  - `Logo string` (optional, references zip entry filename like `"logo.png"`).
- `internal/dataentryconfig/theme.go`: `ValidateThemeManifest(m *ThemeManifest) error` — calls `ValidatePalette(&m.PaletteConfig)`, then validates name/version/author/logo length and shape.
- `internal/dataentry/handlers_theme.go`:
  - `handleAPIThemeExport(w, r)` — reads `AppState`, builds a `ThemeManifest`, writes a zip to `bytes.Buffer`, sets `Content-Type: application/zip` + `Content-Disposition: attachment; filename="<safe-name>.relatheme"`.
  - `handleAPIThemeImport(w, r)` — multipart receive (single `file` field); calls a pure helper `parseThemePackage(bytes []byte) (*ThemeManifest, *ImportedAsset, error)`; on success, persists logo via `saveUserLogo`, returns `{palette: PaletteConfig, logoUrl: string|null}`.
- `internal/dataentry/theme_package.go` (new): `parseThemePackage(bytes []byte) (*ThemeManifest, *ImportedAsset, error)` — pure helper, no `App` access, easy to unit test. Caps zip size, expansion ratio, entry name set, asset size.
- `internal/dataentry/api_v1.go`: register `/api/v1/_theme/export` and `/api/v1/_theme/import`.

Frontend (Vue/TS):

- `frontend/src/api/theme.ts`: add `exportTheme()` (returns `Blob`, triggers a download via `URL.createObjectURL` + anchor click) and `importTheme(file: File)` (returns `{palette, logoUrl}`).
- `frontend/src/views/SettingsView.vue`: new sub-section under the existing Logo card titled **"Theme package"**. Two buttons: Export + Install. Install reuses the file-input pattern with `accept=".relatheme,application/zip"`. After import: `schemaStore.setLogoUrl(result.logoUrl)` (so sidebar updates live); call `loadPaletteState(result.palette, ...)` to populate the existing palette editor. Toast: "Theme installed. Click Save palette to apply colors."
- No store changes — logo flows through the existing `setLogoUrl`; palette flows through the existing editor refs.

Total surface area: **~3 new backend files + 2 modified backend files + 2
modified frontend files**. Effort `m` (already set).

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
logo.<ext>          # optional: PNG / JPEG / SVG / WebP, exists iff manifest.logo is set
```

`theme.yaml` shape (a superset of `palette.yaml`):

```yaml
name: "My Theme"            # required, 1-100 chars
version: "1.0.0"            # required, 1-32 chars (any non-empty)
author: "..."               # optional, 1-100 chars
# colors: identical shape to existing palette.yaml
base: "#1a1a2e"
surface: "#f8fafc"
accent: "#6366f1"
# ... (all PaletteConfig fields: 8 role keys, badges, dark)
logo: "logo.png"            # optional, references zip entry
```

The author of `theme.yaml` always uses the literal entry name `logo.<ext>` so a
parser doesn't need to discover the extension by directory listing.

### Backend export flow

1. `GET /api/v1/_theme/export` reads `AppState`. If a user palette is set use it; otherwise fall back to `cfg.Palette` (the project palette).
2. Compose a `ThemeManifest`:
   - `Name`: defaults to `cfg.App.Name` if not set on the palette/somewhere else.
   - `Version`: `"1.0.0"` (constant for v1).
   - `Author`: empty unless we add a setting later.
   - Palette fields: copied from the source palette.
   - `Logo`: `"logo." + UserLogoExt` if a logo is set, otherwise empty.
3. Marshal to YAML.
4. Build a zip in `bytes.Buffer`:
   - Always write `theme.yaml`.
   - If `UserLogoExt` is set, write `logo.<ext>` containing `UserLogoBytes`.
5. Response headers:
   - `Content-Type: application/zip`
   - `Content-Disposition: attachment; filename="<safeName>.relatheme"` where `safeName` is `Name` with `[^A-Za-z0-9_-]` replaced by `_`, capped to 64 chars; falls back to `theme` if empty.
6. Body = the zip bytes.

### Backend install flow

1. `POST /api/v1/_theme/import` accepts `multipart/form-data` with one file (`file` field, must end in `.relatheme` or `.zip` — content-type sniffed in code).
2. `MaxBytesReader` capped at **5 MiB total** (room for manifest + a 256 KiB logo + headroom).
3. Read the body; parse as zip via `zip.NewReader`.
4. Pass to `parseThemePackage(bytes []byte)`:
   - Reject zips whose **uncompressed total** > 5 MiB or whose **expansion ratio** (declared uncompressed / actual compressed) > 100×.
   - Reject any entry whose normalized name contains `/`, `\`, or `..`. Only flat-file entries with the literal names `theme.yaml` and `logo.<ext>` are accepted; everything else is ignored (so themes with `README.md`, `.DS_Store`, or future entries don't fail).
   - Read `theme.yaml`, parse YAML, run `ValidateThemeManifest`.
   - If `manifest.Logo` is set:
     - Locate the entry `logo.<ext>` exactly. If missing → 400 `"logo referenced in manifest but not present in archive"`.
     - Read up to `MaxUserLogoBytes` + 1 bytes; reject if larger.
     - `sniffLogoMime(bytes)` → must produce one of the allowlist mimes; map back to `ext`. If the sniffed ext doesn't match the manifest's claimed extension we *trust the sniff* and use that.
   - Return `(*ThemeManifest, *ImportedAsset{ext, bytes}, nil)`.
5. Under `mutateState`:
   - If asset present: write logo bytes via `saveUserLogo(bytes, ext)`, recompute hash, update AppState fields.
   - **Do not touch the palette.**
6. Response: `{palette: PaletteConfig, logoUrl: string|null}`.

Why logo persists immediately but palette doesn't: bytes need a stable URL the
browser can fetch, so staging-only would require a tempfile mechanism that
doesn't exist. Palette is JSON, easy to round-trip through the editor; the
existing palette UX already requires explicit Save.

### Manifest type

```go
package dataentryconfig

type ThemeManifest struct {
    Name    string `yaml:"name"`
    Version string `yaml:"version"`
    Author  string `yaml:"author,omitempty"`
    Logo    string `yaml:"logo,omitempty"`
    PaletteConfig `yaml:",inline"`
}

func ValidateThemeManifest(m *ThemeManifest) error {
    if m == nil {
        return errors.New("manifest is nil")
    }
    if l := len(m.Name); l < 1 || l > 100 {
        return fmt.Errorf("name must be 1-100 chars (got %d)", l)
    }
    if l := len(m.Version); l < 1 || l > 32 {
        return fmt.Errorf("version must be 1-32 chars (got %d)", l)
    }
    if l := len(m.Author); l > 100 {
        return fmt.Errorf("author must be 0-100 chars (got %d)", l)
    }
    if m.Logo != "" {
        // Logo entry must be a flat filename matching logo.<ext>.
        if strings.ContainsAny(m.Logo, "/\\") || !strings.HasPrefix(m.Logo, "logo.") {
            return fmt.Errorf(`logo entry must be "logo.<ext>"`)
        }
    }
    return ValidatePalette(&m.PaletteConfig)
}
```

### Frontend install UX

```vue
<section class="settings-card">
  <h3>Theme package</h3>
  <p class="description">
    Bundle the current palette and logo into a portable <code>.relatheme</code>
    file, or install one shared by someone else.
  </p>
  <div class="theme-package-actions">
    <input ref="themeFileInput" type="file"
           accept=".relatheme,application/zip"
           class="file-input-hidden"
           @change="handleThemePicked" />
    <button class="btn btn-secondary btn-sm" @click="handleExport">Export</button>
    <button class="btn btn-primary btn-sm" @click="themeFileInput?.click()">Install</button>
  </div>
</section>
```

Export handler builds an anchor tag, sets `download` to `<safeName>.relatheme`,
clicks it, revokes the object URL.

Install handler POSTs the file, then:

```ts
const result = await importTheme(file)
schemaStore.setLogoUrl(result.logoUrl)        // sidebar updates live
applyImportedPalette(result.palette)           // populate the editor refs
uiStore.success('Theme installed. Click Save palette to apply colors.')
```

`applyImportedPalette` calls the existing `loadPaletteState(result.palette,
paletteRoles.map(r=>r.key), schemaStore.darkDisabled)` and copies the result
into `paletteMode`, `paletteColors`, `paletteBadges`, etc. The same code path
that runs on Settings page mount already does this; install just re-uses it.

### Persistence layout

```text
.rela/
  palette.yaml         # existing, unchanged
  user-defaults.yaml   # existing, unchanged
  theme/
    logo               # logo bytes (existing PR 1)
    logo.ext           # sidecar (existing PR 1)
```

`.rela/theme/manifest.yaml` is **not** persisted. The manifest exists only
inside `.relatheme` zips at export time; on import, only the colors and logo
flow into existing storage. This keeps the install side perfectly aligned with
the existing logo + palette stories.

**Alternatives considered:**

1. **Persist a `theme/manifest.yaml` on import** to capture name/version/author. Rejected: those fields have no display purpose in the SPA today (no "current theme name" UI). When a future ticket needs them, persist then.
2. **Browser-side zip parsing with JSZip.** Rejected: doubles the trust boundary surface area, requires bundling a runtime dep, and brings no UX benefit (server is already handling the upload). Server-side parsing reuses the existing logo/palette validators.
3. **Auto-save palette on import.** Rejected per the user's earlier guidance and consistent with the existing palette UX (explicit Save).
4. **Single endpoint `/api/v1/_theme` with method-based dispatch.** Rejected for clarity: `/_theme/export` (GET) and `/_theme/import` (POST) read more naturally than `GET /_theme` returning a download.
5. **Versioned manifest (`schemaVersion: 1`).** Rejected for v1; defer until we have a second version. The current `version` field is the *theme's* user-facing version, not the schema version.
6. **Reject themes whose manifest references a logo extension we don't allow.** Already covered: `parseThemePackage` always sniffs the actual bytes via `sniffLogoMime`, which is the trust boundary. The manifest's claimed extension is informational only.
7. **Validate that `safeName` matches the user's intent.** Not necessary; download filename is purely cosmetic. Browser's "Save as..." prompt lets users override.

**Files to modify:** see Research section above.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
|---|---|---|
| `.relatheme` upload | Logged-in user via POST | `MaxBytesReader` 5 MiB. Must parse as zip via `zip.NewReader`. Total uncompressed > 5 MiB → reject; expansion ratio > 100× → reject. |
| `theme.yaml` content | Inside zip | YAML parse + `ValidateThemeManifest` (length-bounded name/version/author; logo entry name pattern; embedded `ValidatePalette`). |
| Logo entry inside zip | Inside zip | Entry name is exactly `logo.<sniffed-ext>` flat (no `/`, `\`, `..`). Bytes ≤ `MaxUserLogoBytes` (256 KiB). `sniffLogoMime` must return one of the allowlist mimes — same trust boundary as the direct PUT path. |
| Zip entry names beyond allowlist | Inside zip | Ignored (not enumerated, not extracted). Future themes can add files we don't know about without breaking import. |
| Filename of upload | Multipart header | Ignored. Never reflected in HTML or filenames. |

**Security-Sensitive Operations:**

- **Zip path traversal:** entry names are checked against an allowlist of literal patterns (`theme.yaml`, `logo.*`); anything containing `/`, `\`, or `..` is rejected on the spot. Even allowlist-matching entries are read via `zip.File.Open()` and never used to construct a filesystem path — we hand the bytes to the existing `saveUserLogo` flow which writes to fixed `theme/logo` + `theme/logo.ext` keys.
- **Zip-bomb:** uncompressed-total cap (5 MiB) and per-entry expansion ratio (100×). Both checked before reading entry bodies into memory. The ratio is computed using `zip.File.UncompressedSize64` against the request body length.
- **Stored XSS via SVG logo:** the import path goes through the same logo storage as the merged `_theme/logo` PUT, so the GET path serves bytes with the same `nosniff` + `Content-Security-Policy: sandbox; frame-ancestors 'none'` + `X-Frame-Options: DENY` headers. No new attack surface.
- **YAML deserialization:** `gopkg.in/yaml.v3` (already used) does not execute arbitrary types. We unmarshal into a typed struct; unknown fields are ignored.
- **Authn:** inherits the existing data-entry origin allowlist middleware; same surface as `_palette` / `_theme/logo`.
- **Error responses:** JSON `{error: "<short category>"}`. Categories: `payload_too_large`, `not_a_zip`, `missing_manifest`, `invalid_manifest`, `unsupported_format`, `logo_too_large`, `internal`. No raw bytes / file contents / stack traces in the response.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | How tested |
|---|---|
| 1. Settings shows Export/Install buttons | Vitest mount of `SettingsView`: assert the new card and both buttons are present. |
| 2. Export with palette + logo | Go test in `handlers_theme_package_test.go`: set up app with palette + logo, hit `handleAPIThemeExport`, parse response as zip, assert manifest YAML round-trips and `logo.<ext>` matches the source bytes. |
| 3. Export with no logo | Go test: same setup minus the logo upload; assert the zip has only `theme.yaml`. |
| 4. Install round-trip | Go test: build a zip with `parseThemePackage`-friendly inputs, POST to `handleAPIThemeImport`, assert the response carries the expected palette and logo URL, and that `AppState.UserLogoBytes` was updated. Plus a Playwright e2e: export from one app, install into a fresh app, assert palette editor populated and Save persists. |
| 5. Live sidebar update on install | Playwright e2e: install a theme with a logo, observe `<img>` in sidebar without page reload. |
| 6. Validation matrix | Go table-driven on `parseThemePackage` (pure helper, easy to drive): rows for each error category. Each row asserts the returned error and that no app state was mutated. |
| 7. No new abstractions | Code review + `just arch-lint`. |

**Edge Cases:**

- Manifest with palette ONLY (no logo) — round-trips through export/import.
- Manifest with logo only — palette fields omitted resolve to defaults via `ValidatePalette` (palette is permissive on missing fields).
- Light-only palette vs light+dark palette — both round-trip.
- Zip with extra unrelated entries (`README.md`, `.DS_Store`) — ignored, no error.
- Logo file referenced in manifest but missing in zip → reject with `missing_logo`.
- Logo bytes present in zip but manifest doesn't reference them → ignored (manifest is the source of truth).
- Manifest claims `logo: logo.png` but the actual bytes are SVG → trust the sniff, store as `.svg`.
- Empty zip → reject `missing_manifest`.
- Zip without `theme.yaml` → reject `missing_manifest`.
- `theme.yaml` is a directory entry → reject `invalid_manifest`.
- Non-UTF8 bytes in `theme.yaml` → YAML parser surfaces error → reject `invalid_manifest`.
- Concurrent imports from two browser tabs — last write wins; `mutateState` serializes the writes.
- Two imports in quick succession with the same logo bytes → second hash equals first; no disk thrash.
- 256 KiB logo + 4 MiB of comments in the manifest → zip-total cap (5 MiB) catches it.
- Compressed entry that decompresses to 30 MiB → expansion ratio cap rejects.
- Entry name `../../../etc/passwd` → path-traversal check rejects.
- Entry name `LOGO.PNG` (uppercase) → not matched (case-sensitive); manifest `logo: LOGO.PNG` would similarly fail the pattern check.

**Negative Tests:**

- Upload a non-zip → 400 `not_a_zip`.
- Upload zip without `theme.yaml` → 400 `missing_manifest`.
- `theme.yaml` with malformed YAML → 400 `invalid_manifest`.
- `theme.yaml` with name = "" → 400 `invalid_manifest`.
- `theme.yaml` with name = 200 chars → 400 `invalid_manifest`.
- `theme.yaml` with bad hex color → 400 `invalid_manifest`.
- `theme.yaml` with `logo: ../foo` → 400 `invalid_manifest`.
- Logo entry > 256 KiB → 400 `logo_too_large`.
- Total zip > 5 MiB → 413.
- Zip-bomb (single entry with 100×+ expansion) → 400 `payload_too_large` (or 413 if reaching the body cap first).
- Logo with disallowed mime (e.g. GIF in the zip) → 400 `unsupported_format`.
- GET `/_theme/export` while no palette and no logo → 200 with manifest containing project defaults; this matches the natural "I want a baseline theme" use case.

**Integration test approach:**

- Pure-helper Go tests against `parseThemePackage` for the validation matrix (fast, no HTTP layer).
- HTTP-level Go tests against `handleAPIThemeExport` + `handleAPIThemeImport` for the round-trip (export with state X, import the result, assert state still matches).
- Playwright e2e for the user-facing flow: export from one in-process app, install into a second, verify visible state changes.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| YAML embedding via `yaml:",inline"` causes name/palette field collisions | Low | Medium | `PaletteConfig` field tags are well-defined; explicitly write tests asserting that `name`/`version`/`author`/`logo` are top-level YAML keys distinct from any palette key. |
| Zip-bomb defenses too tight, reject legitimate palettes with long author strings | Low | Low | 5 MiB total + 100× ratio is generous: a manifest is ~2 KB; logos are ≤256 KiB. Real headroom is >19×. |
| Logo size cap differs between `parseThemePackage` and direct PUT | Low | Medium | Both paths reuse `MaxUserLogoBytes` constant. A test asserts both reject at the same boundary. |
| Browser caches an old export | Low | Low | We don't cache exports — `Cache-Control: no-store` on the export response. |
| Import succeeds at logo persist but palette response shape changes downstream | Low | Medium | The palette response shape mirrors the existing `userPalette` field returned by `/api/v1/_settings`. Type-shared: extend `frontend/src/api/theme.ts` to re-use `PaletteConfig`. |
| User imports a theme on a workspace where colors are project-locked | Low | Low | Out of scope for this PR — the same constraint exists for the direct palette PUT today. The install simply stages; user can't save if locked. |

**Effort:** **m** — already set on TKT-WPKW. Backend ~250 LOC + tests; frontend
~80 LOC + tests; one new Go file (`theme_package.go`) and one new manifest type.
Realistic estimate: 1 working day.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — add a "Theme packages" subsection under data-entry settings docs explaining export/import format.
- [x] ~~CLI help text~~ (N/A: no CLI surface).
- [x] ~~CLAUDE.md~~ (N/A: no new architecture concept).
- [x] ~~README.md~~ (N/A: feature is internal to data-entry).
- [x] API docs — document `/api/v1/_theme/export` and `/api/v1/_theme/import` alongside `_palette` and `_theme/logo`.

## Design Review

- [x] Run `/design-review` before starting implementation — used `/crit:crit`. Plan went through one round; the only inline question (font/CSS scope) was addressed by the prior umbrella restructuring before this rewrite, so no plan-stage review responses were filed.
- [x] All critical/significant findings addressed in plan — none surfaced at plan stage; implementation review yielded RR-84YM, RR-YEVY, RR-0PTF, RR-5QTT, RR-U2S9 (significant) + RR-MP1R, RR-7P3O (minor) + RR-5EHQ, RR-YMSC, RR-OMEN (nit) — all addressed.

**Design Review Findings:** None at plan stage. Implementation findings tracked under TKT-WPKW has-review-response.
