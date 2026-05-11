---
id: PLAN-M2UZ
type: planning-checklist
title: 'Planning: Custom UI font upload for data-entry'
status: pending
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope (PR 2 of the three-PR theme system split):**

- Backend: `GET / PUT / DELETE /api/v1/_theme/font`. Storage at `.rela/theme/font.<ext>` via the existing `state.KV` (Delete added by PR 1).
- Mime / magic-byte validation: WOFF2 (`wOF2`), WOFF (`wOFF`), TTF (`\x00\x01\x00\x00` / `OTTO`), OTF (`OTTO`). Max 2 MiB.
- Family name: stored in a small sidecar `theme/font.meta` (YAML or single-line `family\n<name>` plain text — see Approach). Validated 1–64 chars `[A-Za-z0-9 _-]`.
- `AppState` additions: `UserFontBytes`, `UserFontExt`, `UserFontFamily`, `UserFontHash`. Loaded at app boot, mutated under `mutateState` on PUT/DELETE.
- `_sidebar` and `_settings` responses gain `fontUrl` + `fontFamily` (or a single `font: {url, family}` object — see Approach).
- Frontend: `frontend/src/api/theme.ts` extends with `uploadFont(file, family)`, `removeFont()`, `FontInfo` type.
- Frontend: Settings → Appearance gets a Font sub-section with file picker + family-name input + preview text + Remove button.
- Frontend: `schemaStore` gains `fontUrl` / `fontFamily` refs + `setFont(url, family)`. A new composable `applyFont` in `composables/useFont.ts` injects an `@font-face` `<style>` and toggles `:root` `--ui-font`.
- `App.vue`: global `font-family` becomes `var(--ui-font, <existing stack>)`.
- Cache-busting via SHA-256 content hash, same approach as the logo.

**Out of scope (deferred):**

- Web fonts from external URLs (Google Fonts, etc.) — local upload only.
- Multiple weights / styles (italic, bold variants) — single file = single weight; CSS handles synthesized variants.
- Font as part of a `.relatheme` package (PR 3 / TKT-WPKW).
- Per-component font choices (mono vs body) — one font, one slot.
- Variable-font subsetting / pre-processing.

**Acceptance Criteria:**

1. **Settings shows a Font control.** Test: Vitest mount of `SettingsView` asserts the file picker (`accept=".woff2,.woff,.ttf,.otf,font/*"`), the family-name input, the preview text element, and Upload + Remove buttons.
2. **Upload persists + applies live.** Test (e2e): pick a `.woff2` fixture, type a family, click Upload. Assert (a) `_settings.fontUrl` populates on next fetch; (b) `getComputedStyle(document.body).fontFamily` includes the family name; (c) `<style data-rela-font>` exists in `<head>`.
3. **Remove reverts to system stack.** Test (e2e): with a font set, click Remove, assert `--ui-font` is unset and the family name is no longer in `getComputedStyle(body).fontFamily`.
4. **Validation matrix.** Go table-driven: rows for 2 MiB + 1 byte (413), TTF magic / OTF magic / WOFF magic / WOFF2 magic (accepted), random bytes claiming font extension (rejected on magic-byte mismatch), missing form field (400), bad family name (400). On any failure the existing font is unchanged.
5. **Family name validation.** `family` must match `/^[A-Za-z0-9 _-]{1,64}$/`. Test: empty / 65-char / `<script>` / Unicode chars all rejected at the API.
6. **Licensing notice.** Visible caption next to the upload control: "Only upload fonts you are licensed to redistribute / use locally." Vitest mount assertion.
7. **Persistence across restart.** Manual: upload, kill server, restart; `_settings.fontUrl` returns the same hash; bytes served byte-for-byte.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **`state.KV` interface** (added in PR 1, TKT-WN7O): Get / Put / Delete. Already supports the Delete contract we need. **Important: PR 2 branches off `develop`, so this method only exists once PR 1 merges.** If PR 1 stalls beyond PR 2, rebase will pull in the interface change.
- **Logo PR (TKT-WN7O) plumbing** as the reference template:
  - `internal/dataentry/theme_logo.go` — load/save/delete + `LogoURL()` accessor. Mirror the structure for fonts.
  - `internal/dataentry/handlers_theme.go` — multipart parsing, `MaxBytesReader`, `mutateState`, hash computation, response shape. Mirror.
  - `internal/dataentry/handlers_theme_test.go` — fixture builders, validation table. Mirror.
  - `frontend/src/api/theme.ts` — typed client. Extend.
  - `frontend/src/stores/schema.ts` — `logoUrl` + `setLogoUrl`. Add `fontUrl`, `fontFamily`, `setFont`.
  - `frontend/src/views/SettingsView.vue` — Logo card pattern. Add a sibling Font card.
- **`http.DetectContentType` does NOT identify fonts** — sniffs the first 512 bytes against image / text / video / audio signatures. Returns `application/octet-stream` for fonts. We have to magic-byte them ourselves:
  - **WOFF2:** first 4 bytes `wOF2` (`0x77 0x4F 0x46 0x32`).
  - **WOFF:** first 4 bytes `wOFF` (`0x77 0x4F 0x46 0x46`).
  - **TTF:** first 4 bytes `\x00\x01\x00\x00` (TrueType outlines) or `true` (Apple) or `typ1` (rare PostScript).
  - **OTF:** first 4 bytes `OTTO` (CFF outlines).
  - References: <https://www.w3.org/TR/WOFF2/#table-overall-file-structure>, <https://learn.microsoft.com/en-us/typography/opentype/spec/otff>.
- **`@font-face` injection in JS:** standard pattern is to insert a `<style>` element with the rule into `<head>`. The browser resolves the URL and applies the font once any element references the family. No need for `FontFace` API (more code, no benefit here).
- **Existing `--*` CSS vars in App.vue:** the palette story already uses `--accent-color`, `--text-color`, etc. Add `--ui-font` as a sibling.
- **Family-name validation regex:** `/^[A-Za-z0-9 _-]{1,64}$/`. The CSS spec allows wider names with quoting, but limiting to ASCII identifier chars eliminates quoting / escape ambiguity in the inline `<style>` we generate. Users with weirder fonts can rename via the input field — we never auto-derive from the file.
- **Font sniff library** — none in stdlib, none popular in third-party Go. Magic-byte check is 8 lines of code.

**Files to modify:**

Backend (Go):

- `internal/dataentry/theme_font.go` (new): `loadUserFont`, `saveUserFont`, `deleteUserFont`, `sniffFontExt`, `hashFontBytes`, `FontURL()` on AppState — all mirroring `theme_logo.go`.
- `internal/dataentry/app.go`: extend `AppState` with `UserFontBytes`, `UserFontExt`, `UserFontFamily`, `UserFontHash`. Boot path calls `loadUserFont` next to `loadUserLogo`.
- `internal/dataentry/handlers_theme.go`: add `handleAPIThemeFont` with the same GET / PUT / DELETE shape. Multipart PUT carries both the file and a `family` form field. **Refactor opportunity:** lift shared multipart-upload helpers if the duplication starts hurting; do not preempt the abstraction.
- `internal/dataentry/api_v1.go`: register `/api/v1/_theme/font` route.
- `internal/dataentry/api_v1.go::handleV1Sidebar` + `handlers_api.go::handleAPIGetSettings`: extend response with `font: {url, family}`.

Frontend (Vue/TS):

- `frontend/src/api/theme.ts`: add `uploadFont(file, family) → {fontUrl, fontFamily}`, `removeFont()`. Reuse `LogoUploadError` shape (or rename to `ThemeUploadError`).
- `frontend/src/api/settings.ts`: extend `SettingsData` with `font?: {url, family} | null`.
- `frontend/src/types/config.ts`: extend `SidebarData` with same.
- `frontend/src/stores/schema.ts`: add `fontUrl`, `fontFamily`, `setFont(url, family) | clearFont()`.
- `frontend/src/composables/useFont.ts` (new): `applyFont(url, family)` injects/updates the `<style data-rela-font>` element and sets `--ui-font`. `clearFont()` removes both.
- `frontend/src/App.vue`: watch `schemaStore.fontUrl` / `fontFamily`, call `applyFont` / `clearFont` on changes. Update global `font-family` to `var(--ui-font, -apple-system, ...)`.
- `frontend/src/components/common/Sidebar.vue::loadSidebar`: push `data.font` into `schemaStore.setFont` (mirror `setLogoUrl`).
- `frontend/src/views/SettingsView.vue`: new Font card with file picker, family input, preview text, Upload + Remove. Mirror SettingsView's Logo card.

Total surface area: ~5 new backend lines + ~3 modified backend files / ~6
frontend files. Effort `s`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Storage layout

```text
.rela/
  theme/
    logo            # PR 1 (existing)
    logo.ext        # PR 1 (existing)
    font            # bytes
    font.ext        # 3-5 bytes: "woff2" | "woff" | "ttf" | "otf"
    font.family     # 1-64 bytes: the family name as raw UTF-8
```

The decision to store family in its own sidecar (rather than
`theme/manifest.yaml` as the original umbrella plan suggested) keeps the on-disk
shape symmetrical with the logo and avoids having two mechanisms (sidecar files
+ a manifest) for the same kind of metadata. PR 3 (the `.relatheme` zip) creates
the manifest only at export time.

### Endpoint behavior

#### `GET /api/v1/_theme/font`

- 404 if `state.UserFontExt == ""`.
- 200 with body = bytes, `Content-Type` per `UserFontExt`:
  - `woff2` → `font/woff2`
  - `woff` → `font/woff`
  - `ttf` → `font/ttf`
  - `otf` → `font/otf`
- `X-Content-Type-Options: nosniff`.
- `Cache-Control: public, max-age=86400, immutable` (URL carries content hash).
- **No CSP sandbox** (unlike logos): a font response isn't an executable surface. We do set `Cross-Origin-Resource-Policy: same-origin` so cross-origin pages can't `@font-face` from our server.

#### `PUT /api/v1/_theme/font`

- Body: `multipart/form-data` with `file` (the font) and `family` (the name).
- Step 1: `MaxBytesReader` at `MaxUserFontBytes + 16 KiB` (= 2 MiB + envelope headroom, mirrors logo).
- Step 2: `r.ParseMultipartForm`.
- Step 3: validate `family`: `^[A-Za-z0-9 _-]{1,64}$`.
- Step 4: read file bytes; verify size ≤ 2 MiB.
- Step 5: magic-byte check → derive extension. Reject anything not in the allowlist.
- Step 6: under `mutateState`, persist bytes + `font.ext` + `font.family`. Update AppState.
- Response: `{ok: true, fontUrl: "/api/v1/_theme/font?v=<hash>", fontFamily: "<name>"}`.

#### `DELETE /api/v1/_theme/font`

- Under `mutateState`: `kv.Delete` on all three keys (idempotent). Clear AppState fields.
- Returns 204.

### Magic-byte detection

```go
func sniffFontExt(b []byte) string {
    if len(b) < 4 {
        return ""
    }
    switch {
    case bytes.HasPrefix(b, []byte("wOF2")):
        return "woff2"
    case bytes.HasPrefix(b, []byte("wOFF")):
        return "woff"
    case bytes.HasPrefix(b, []byte{0x00, 0x01, 0x00, 0x00}),
         bytes.HasPrefix(b, []byte("true")),
         bytes.HasPrefix(b, []byte("typ1")):
        return "ttf"
    case bytes.HasPrefix(b, []byte("OTTO")):
        return "otf"
    }
    return ""
}
```

### Frontend application

`composables/useFont.ts`:

```ts
const STYLE_ID = 'rela-user-font'

export function applyFont(url: string, family: string) {
  let el = document.getElementById(STYLE_ID) as HTMLStyleElement | null
  if (!el) {
    el = document.createElement('style')
    el.id = STYLE_ID
    document.head.appendChild(el)
  }
  // family is server-validated to [A-Za-z0-9 _-], so quoting is safe
  // without escaping; URL is server-controlled (internal route).
  el.textContent =
    `@font-face { font-family: "${family}"; src: url("${url}"); font-display: swap; }`
  document.documentElement.style.setProperty(
    '--ui-font',
    `"${family}", -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`,
  )
}

export function clearFont() {
  document.getElementById(STYLE_ID)?.remove()
  document.documentElement.style.removeProperty('--ui-font')
}
```

`App.vue` style update:

```css
body {
  font-family: var(--ui-font, -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif);
}
```

`App.vue` script: `watch([() => schemaStore.fontUrl, () =>
schemaStore.fontFamily], ([url, family]) => { url && family ? applyFont(url,
family) : clearFont() }, {immediate: true})`.

### Settings UI

```vue
<section class="settings-card">
  <h3>Font</h3>
  <p class="description">
    Upload a custom UI font. WOFF2, WOFF, TTF, or OTF. Max 2 MiB.
  </p>
  <p class="muted-caption">
    Only upload fonts you are licensed to redistribute or use locally.
  </p>
  <div class="font-preview-row">
    <div
      class="font-preview-frame"
      :style="stagedFontFamily ? { fontFamily: stagedFontFamily } : {}"
    >
      The quick brown fox jumps over the lazy dog. 0123456789
    </div>
    <div class="font-actions">
      <input type="text" v-model="stagedFamilyName" placeholder="Family name" maxlength="64" />
      <input ref="fontFileInput" type="file" accept=".woff2,.woff,.ttf,.otf,font/*" hidden @change="handleFontPicked" />
      <button class="btn btn-secondary btn-sm" @click="fontFileInput?.click()">Choose font</button>
      <button
        class="btn btn-primary btn-sm"
        :disabled="!stagedFontFile || !stagedFamilyName.trim() || uploadingFont"
        @click="handleFontUpload"
      >Upload</button>
      <button v-if="fontUrl" class="btn btn-danger btn-sm" :disabled="removingFont" @click="handleFontRemove">Remove</button>
    </div>
  </div>
</section>
```

Preview behavior: the staged file is loaded into a temp `FontFace` instance via
the `FontFace` API (`new FontFace(family, src)` then `document.fonts.add(...)`),
so the preview frame can render in the picked font even before Upload. After
Upload, the persisted `applyFont` takes over and the temp instance is dropped.

### Persistence layout

The on-disk story is intentionally identical to the logo so PR 3 (`.relatheme`)
can iterate over `theme/<asset>{,.ext,.family?}` uniformly.

**Alternatives considered:**

1. **Family in `theme/manifest.yaml` (umbrella plan).** Rejected for this PR: introduces a YAML manifest that nothing else needs, and PR 3 will create one anyway. Two manifests vs. one symmetrical sidecar story — symmetry wins.
2. **Drop family input, derive from filename.** Rejected: filenames are user-uncontrolled noise (`MyFont-Regular-v2.woff2`); the user typing the family name eliminates ambiguity.
3. **Use `FontFace` API instead of injected `<style>`.** Both work; `<style>` injection composes with CSS variables more cleanly and matches how the rest of the SPA handles theming. `FontFace` API is used only for the staged preview where we need synchronous loading.
4. **Allow Unicode family names.** Rejected for v1: harder to safely embed in inline CSS without escape risk. The user can name it whatever they like during display ("Helvetica Sans"); we just store ASCII.
5. **Sniff via `mime.TypeByExtension(filename)`.** Rejected: trusts user-supplied filename. Magic bytes are the trust boundary.
6. **Reuse PR 1's `writeLogoTooLarge` helper directly.** Rejected: too coupled. Mirror the pattern with `writeFontTooLarge` to keep error wording specific. Lift to a shared helper only if a third asset arrives.

**Files to modify:** see Research section above.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
|---|---|---|
| Multipart `file` field | PUT body | `MaxBytesReader` 2 MiB + 16 KiB envelope. After parse, `len(bytes) > 2 MiB` → 413. |
| Font bytes | Inside multipart | Magic-byte allowlist: `wOF2` / `wOFF` / `\x00\x01\x00\x00` / `true` / `typ1` / `OTTO`. No fallback. |
| `family` form field | PUT body | `^[A-Za-z0-9 _-]{1,64}$`. Empty / overlong / invalid char → 400. |
| Filename in upload | Multipart header | Ignored. Always renamed to sniffed extension. |
| Sidecar `font.ext` | `.rela/theme/font.ext` on boot | Read back validated as one of `{woff2,woff,ttf,otf}`. Anything else → treat as no font, log warning. |
| Sidecar `font.family` | `.rela/theme/font.family` on boot | Same regex re-validation. Anything else → no font, log warning. |

**Security-Sensitive Operations:**

- **Inline `<style>` injection** (`composables/useFont.ts`): the `family` is server-validated, so no quote-escape risk in `font-family: "<family>"`. The URL is a server-internal route (`/api/v1/_theme/font?v=<hex hash>`), never user-controllable in shape. No `eval`, no `innerHTML`-derived-from-input.
- **CSS injection via family**: even if validation drifts, the format-string surface in the inline style is one place. Worst case (validation lapses): an attacker who can already PUT to the local-only API can also break their own SPA — no privilege escalation.
- **Cross-origin font theft**: not a concern (no auth secrets in fonts), but we set `Cross-Origin-Resource-Policy: same-origin` so the font response can't be loaded by foreign pages.
- **Path traversal**: keys are static literals; user input never reaches the path. Same as logo PR.
- **Image-decoder vulnerabilities equivalent**: we never decode the font server-side. No `freetype`, no `image/font` library. Browsers do the parsing; that's the same trust surface as any web font.
- **DoS via large upload**: 2 MiB cap. `MaxBytesReader` enforced before parse.
- **Server-side magic-byte inspection only on first 4 bytes** — no full-format validation. A malformed font that passes the 4-byte check will load and the browser will reject it on render. Acceptable because the user's own browser is the victim of malformed data they uploaded themselves.

**Error responses:** JSON `{error: "<category>"[, maxBytes: 2097152]}` for 413.
No bytes / hex dumps / stack traces in the body.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | How tested |
|---|---|
| 1. Settings shows Font control | `frontend/src/views/SettingsView.test.ts`: mount + assert presence of file input, family input, preview text, Upload + Remove. |
| 2. Upload persists + applies live | Playwright e2e in `/e2e/`: pick fixture WOFF2 + family, click Upload, await `<style data-rela-font>`, assert `getComputedStyle(body).fontFamily.includes(family)`. |
| 3. Remove reverts to system stack | Playwright e2e: with font set, click Remove, assert `<style data-rela-font>` gone, assert `getComputedStyle(body).fontFamily` no longer contains family. |
| 4. Validation matrix | Go table-driven test in `internal/dataentry/handlers_theme_font_test.go`: covers 2 MiB+1, each magic-byte (accept), GIF bytes (reject), missing field, bad family. Assert state unchanged on rejection. |
| 5. Family name validation | Go table-driven: empty, 65 chars, `<script>`, `My Font`, `Inter-Display`, `name with quotes "x"` — each row asserts 200 vs 400 + that AppState reflects only successes. |
| 6. Licensing notice | Vitest mount: `expect(wrapper.text()).toContain('Only upload fonts you are licensed')`. |
| 7. Persistence across restart | Manual: e2e or `curl`-driven smoke test in implementation. |

**Edge Cases:**

- Upload exactly 2 MiB → accepted.
- Upload 2 MiB + 1 → 413.
- Two concurrent PUTs (multi-tab) → `mutateState` serializes, last write wins, hash converges on next fetch.
- Upload, immediately rename family via second PUT → bytes preserved, only metadata changes if the family field is the only thing different (current path always rewrites bytes; could optimize later, but the simpler "PUT replaces all" is fine).
- App restart with `font` + `font.ext` present but `font.family` missing → log warning, treat as no font.
- App restart with valid `font` + `font.ext` + `font.family` containing invalid chars → same — no font, log warning.
- WOFF2 with EOF padding → magic check sees prefix only; bytes pass through opaquely.
- Browser caches old font after replacement → URL hash changes; cache miss; new bytes fetched.
- User unloads page mid-upload → standard fetch abort; no partial state because `mutateState` is the gate.

**Negative Tests:**

- POST a PNG → 400 unsupported_format.
- POST `text/plain` claiming to be `.ttf` → magic check fails, 400.
- POST without `family` field → 400 missing_family.
- POST with empty `family` → 400 invalid_family.
- POST with `family="<script>alert(1)</script>"` → 400 invalid_family (regex rejects `<`, `>`).
- POST with `family` of 65 chars → 400.
- DELETE before upload → 204 (idempotent).
- GET before upload → 404 + JSON error.
- PATCH method → 405.

**Integration test approach:**

- Go test driving full PUT → GET → DELETE → GET cycle against an in-memory `state.KV`. Asserts byte-for-byte preservation, hash propagation, family round-trip, 404 after DELETE.
- Vitest test for `frontend/src/api/theme.ts::uploadFont` with mocked fetch — confirms `FormData` shape (`file` + `family`) and response handling.
- Vitest test for `composables/useFont.ts::applyFont` / `clearFont` — assert `<style data-rela-font>` is created/updated/removed and `--ui-font` is set/cleared.
- Playwright e2e for the round-trip flow.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Branch-off-`develop` while PR 1 unmerged ⇒ rebase conflicts | High | Low | Limit textual conflicts: each new asset adds *next to* the logo's lines, never modifying them. Conflict resolution = keep both blocks. |
| `state.KV.Delete` only exists on `feat/theme-logo` ⇒ won't compile on `develop` | High | Low | If PR 1 hasn't merged at PR 2's commit time, cherry-pick the `state.KV.Delete` change into PR 2 and let one of them rebase later. |
| Browser fails to load the font (corrupt magic bytes pass our 4-byte check) | Medium | Low | Document that the user sees the system font fallback if their font is malformed. No server-side fix; we don't decode fonts. |
| Family name with embedded quotes breaks injected `<style>` | Low | Medium | Server-side regex bans `"`; even if it slipped through, the consequence is a malformed CSS rule on the user's own browser. |
| `font-display: swap` causes layout shift on first paint | High | Low | This is the standard tradeoff (vs `block` which delays render or `optional` which may skip the font). Acceptable; matches Google Fonts default. |
| FontFace API for staged preview unsupported on old Safari | Low | Low | Feature-detect; fall back to "no preview" with a static text label. |
| Magic-byte allowlist misses an edge case (TrueType collection `ttcf` etc.) | Low | Low | TTC is a font *collection* (multiple fonts in one file); not in the spec for `@font-face` single-family use. Reject with the same error; if a real user need surfaces, add it. |

**Effort:** **s** — already set on TKT-KE0C. Backend ~150 LOC + tests; frontend
~100 LOC + composable + tests. Realistic estimate: half a working day.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — extend the "Custom logo" section (added in PR 1) with a sibling "Custom font" subsection.
- [x] ~~CLI help text~~ (N/A: no CLI surface).
- [x] ~~CLAUDE.md~~ (N/A: deferred to PR 3 with the broader theme system pass).
- [x] ~~README.md~~ (N/A: feature is internal to data-entry).
- [x] API docs — document `/api/v1/_theme/font` (GET / PUT / DELETE) alongside `_palette` and `_theme/logo`.

## Design Review

- [x] Run `/design-review` before starting implementation — will use `/crit:crit` per project pattern (see PR 1's PLAN-65IV).
- [ ] All critical/significant findings addressed in plan — pending crit round.

**Design Review Findings:** TBD (will be filled in after `/crit:crit` round).
