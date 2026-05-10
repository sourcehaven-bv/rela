---
id: PLAN-65IV
type: planning-checklist
title: 'Planning: Custom logo upload for data-entry sidebar branding'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope (PR 1 of the three-PR theme system split):**

- Backend: `GET / PUT / DELETE /api/v1/_theme/logo`. Storage at `.rela/theme/logo.<ext>` via the existing `state.KV` abstraction.
- Mime / size validation: `image/png`, `image/jpeg`, `image/svg+xml`, `image/webp`. Max 256 KiB.
- SVG safety: rendered via `<img src>` only — no server-side sanitization. Browser sandbox is the trust boundary (no script execution, no external fetches, no DOM access for `<img>`-loaded SVGs across all major browsers).
- One small dependency change: add `Delete(ctx, key) error` to `state.KV` interface and `state.FSKV` impl. The Get/Put-only interface is awkward and PR 2 + PR 3 will both want Delete too.
- AppState additions: `UserLogoExt string` (empty = no logo). Loaded at app boot, mutated under `mutateState` on PUT/DELETE.
- New endpoint reports its result via `GET /api/v1/_settings` so the existing `getSettings()` call exposes the logo URL (or null) to the SPA.
- Frontend: `frontend/src/api/theme.ts` (new): typed clients for upload / delete / build URL.
- Frontend: Settings → Appearance gets a Logo sub-section with file picker, preview, Remove button.
- Frontend: `Sidebar.vue` renders `<img class="logo-img">` when a logo is set, falls back to the existing text otherwise.
- Cache-busting: response includes a content-hash query param so updates render instantly without manual cache control.

**Out of scope (deferred, see TKT-WPKW for context):**

- Multi-resolution rasters (`srcset` / `<picture>` / `@2x`) — the manifest can grow `logo: string` → `logo: object` later without breaking existing themes.
- Logo as part of a `.relatheme` package (PR 3 / TKT-WPKW).
- Custom favicon — different problem; this only touches sidebar branding.
- Sanitizing SVG content server-side — relies on browser `<img>` sandbox.
- Drag-and-drop upload — standard `<input type="file">` only.

**Acceptance Criteria:**

1. **Settings shows a Logo control.** Test: open Settings → Appearance, assert the section has a file picker (`accept="image/png,image/jpeg,image/svg+xml,image/webp"`), an image preview area, and a Remove button (Vitest mount test against `SettingsView`).
2. **Upload persists + immediately replaces sidebar text.** Test (e2e): pick a 100×100 PNG, click Upload, observe a "Logo updated" toast and an `<img>` element in the sidebar header instead of the text.
3. **Remove deletes the file + reverts to text fallback.** Test (e2e): with a logo set, click Remove, observe text reappears and a follow-up `GET /api/v1/_theme/logo` returns 404.
4. **Size / mime validation.** Test (Go table-driven): rows for 257 KiB PNG (reject), 256 KiB PNG (accept), `application/octet-stream` body sniffing as PNG bytes (accept on sniffed type), bare text bytes claiming `image/png` (reject), GIF (reject — not in allowlist). Each failing row asserts 4xx + specific error body and that the existing logo file is unchanged.
5. **SVG `<script>` cannot execute.** Test (Playwright e2e): upload a fixture SVG containing `<script>alert(1)</script>` + `<image xlink:href="https://example.invalid/leak"/>`. Assert no `dialog` event fires, no network request to the external URL, and the visible logo renders (sandbox does not blank the SVG, it just neutralizes scripts).
6. **Sidebar layout works in expanded and collapsed states.** Test (Vitest snapshot or e2e): with sidebar at full width, `.logo-img` is visible and constrained to fit; in collapsed state (60px), the logo is hidden by the existing `.collapsed .logo { display: none }` rule (the image inherits the same parent class).

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **`state.KV` abstraction** at `internal/state/state.go:18`: `Get(ctx, key) ([]byte, error)`, `Put(ctx, key, data) error`. Already used for `palette.yaml`, `user-defaults.yaml`, `ui-state.json`. **Lacks `Delete`** — we add it (one method on one interface, one impl in `FSKV`; `RootedFS.Remove` already exists at `internal/storage/rooted.go:194`).
- **Existing palette save flow** (`saveUserPalette` at `app.go:542`, `handleAPISavePalette` at `handlers_api.go:907`): the model to mirror — load on app boot into `AppState`, mutate via `mutateState` so the snapshot is consistent for concurrent readers.
- **Existing settings endpoint** (`GET /api/v1/_settings`, `handleAPIGetSettings` at `handlers_api.go:735`): we extend its response with a `logoUrl` field (string or null) instead of inventing a new GET endpoint just for SPA bootstrapping.
- **Existing `mutateState` helper**: takes a `func(*AppState)` callback under writer mutex; readers hit `atomic.Pointer[State]` so palette/logo updates land coherently.
- **Sidebar branding today** (`Sidebar.vue:25`): `appName = computed(() => sidebarAppName.value || schemaStore.app.name)` — a single computed in the header. Adding a logo is a one-line v-if/else over that.
- **Mime sniffing**: stdlib `http.DetectContentType` (looks at first 512 bytes; correctly identifies PNG / JPEG / WebP / SVG-as-XML). Sufficient — no third-party lib.
- **SVG sandboxing in `<img>`:** spec'd in HTML Living Standard, implemented by Chrome/Firefox/Safari/WebKit. Scripts in `<img>`-loaded SVGs do NOT run; external resource references (`<image href>`, CSS `url()`) do NOT load. References:
  - HTML spec §4.8.4.4: "If the image is an SVG document, the user agent must not honor any requests to load external resources..."
  - MDN: <https://developer.mozilla.org/en-US/docs/Web/SVG/Tutorial/SVG_Image_Tag>
  - This is a well-trodden path; GitHub, GitLab, etc. use the same trust model for user-uploaded SVGs displayed via `<img>`.
- **No new third-party dependencies** are needed: zip is for PR 3 only; SVG sanitizers (bluemonday) avoided per the design discussion in the crit thread.
- **Reference implementation: GitHub's user avatars**: stored as opaque blobs, served via dedicated endpoint with content-hash cache busting (`avatars1.githubusercontent.com/.../<hash>`). Same pattern.
- **Existing ETag / Cache-Control patterns** in dataentry: none consistently applied to non-static assets. We'll use `Cache-Control: public, max-age=86400` plus a content-hash in the URL — the URL changes on every save, so stale caches are impossible.

**Files to modify:**

Backend (Go):

- `internal/state/state.go`: add `Delete(ctx, key) error` to the `KV` interface; impl on `*FSKV` calls `s.fs.Remove(key)` and returns `nil` if `os.IsNotExist`. Update test file with the new contract test row.
- `internal/dataentry/app.go`:
  - Add `userLogoFile = "theme/logo"` constant. Logo extension lives in a tiny sidecar metadata file `theme/logo.ext` (a 3–4 byte file containing `png` / `jpeg` / `svg` / `webp`) so the bytes file's extension is decoupled from `kv` keys (which don't carry MIME info).
  - Add `loadUserLogo() (bytes []byte, ext string, err error)` and `saveUserLogo(bytes []byte, ext string) error` and `deleteUserLogo() error` helpers, all under `kv`.
  - Add `UserLogoBytes []byte` and `UserLogoExt string` and `UserLogoHash string` to `AppState`. (Bytes in-memory: small, ≤256 KiB, avoids re-reading disk on every GET.)
  - Compute `UserLogoHash` on load + on every save (e.g. `sha256.Sum256(bytes)[:12]` hex).
- `internal/dataentry/handlers_api.go`:
  - `handleAPIThemeLogoCRUD` routes by method:
    - `GET`: serves bytes with `Content-Type` from `UserLogoExt`, `Cache-Control: public, max-age=86400, immutable`. 404 if no logo.
    - `PUT`: parses `multipart/form-data` (form field `logo`), validates size + sniffed mime, writes via `mutateState` → `saveUserLogo`. Returns `{ok: true, logoUrl: "/api/v1/_theme/logo?v=<hash>"}`.
    - `DELETE`: clears state + calls `deleteUserLogo`. Returns 204.
  - Extend `APISettingsData` (and the resolver in `handleAPIGetSettings`) with `LogoUrl *string` (nil → no logo).
- `internal/dataentry/api_v1.go`: register `mux.HandleFunc("/api/v1/_theme/logo", a.handleAPIThemeLogoCRUD)`.

Frontend (Vue/TS):

- `frontend/src/api/theme.ts` (new): `uploadLogo(file: File)`, `removeLogo()`, `buildLogoUrl(hash: string | null)`. Returns the new `logoUrl` from the server response so the store can update without re-fetching settings.
- `frontend/src/api/settings.ts`: extend `SettingsData` with `logoUrl?: string | null`.
- `frontend/src/stores/schema.ts`: add `logoUrl` ref + setter; `applyLogo(url)` updates the ref so the Sidebar reactively re-renders.
- `frontend/src/views/SettingsView.vue`: new `<section>` "Logo" inside the existing Appearance area:
  - Hidden `<input type="file" accept="image/png,image/jpeg,image/svg+xml,image/webp">` triggered by a "Choose image" button.
  - Image preview (`<img :src="logoPreviewUrl">` showing the current logo or the picked-but-not-yet-uploaded file via `URL.createObjectURL`).
  - "Upload" + "Remove" buttons, disabled while in flight.
  - A 1-line caption with the size/format constraints and licensing reminder.
- `frontend/src/components/common/Sidebar.vue`:
  - Replace the current `<RouterLink class="logo">{{ appName }}</RouterLink>` with `<RouterLink class="logo">`, inner content is `<img v-if="logoUrl" :src="logoUrl" :alt="appName" class="logo-img"> <span v-else>{{ appName }}</span>`. Both branches use the same `.logo` class so the existing `.collapsed .logo { display: none }` rule still hides them.
  - Add `.logo-img { max-height: 28px; max-width: 100%; object-fit: contain; display: block; }` next to the existing `.logo` rules.

Total surface area: ~6 backend files / ~5 frontend files. Effort `s`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Storage layout in `.rela/`

```text
.rela/
  palette.yaml          # existing
  user-defaults.yaml    # existing
  theme/
    logo                # the bytes (no extension — extension lives in sidecar)
    logo.ext            # 3-4 bytes: "png" | "jpeg" | "svg" | "webp"
```

The reason for the sidecar `.ext` file: `state.KV` keys are opaque strings;
storing extension as part of the filename ("logo.png" vs "logo.svg") would
require directory listing to discover what extension is present after a restart.
A 4-byte sidecar file is simpler than introducing directory enumeration to `KV`.

### Endpoint behavior

#### `GET /api/v1/_theme/logo`

- 404 if `state.UserLogoBytes == nil`.
- 200 with body = bytes, `Content-Type` = inferred from `UserLogoExt`:
  - `png` → `image/png`
  - `jpeg` → `image/jpeg`
  - `svg` → `image/svg+xml`
  - `webp` → `image/webp`
- Headers: `Cache-Control: public, max-age=86400, immutable` (URL contains content hash, so any update is a different URL).
- `Content-Security-Policy: sandbox` — extra-strict CSP scoped to the response. Even if a browser tried to interpret the SVG response as a top-level navigation, sandbox would prevent script execution.

#### `PUT /api/v1/_theme/logo`

- Body: `multipart/form-data` with one field `logo`.
- Step 1: limit reader to 257 KiB. If body exceeds 256 KiB after reading, 413 Payload Too Large.
- Step 2: read all bytes into memory.
- Step 3: `http.DetectContentType(bytes[:512])` → must match allowlist `{image/png, image/jpeg, image/svg+xml, image/webp}`. Otherwise 400 with `code: "unsupported_format"`.
- Step 4: derive extension from sniffed mime.
- Step 5: under `mutateState`, write bytes to `theme/logo` and extension to `theme/logo.ext`, compute hash, update `AppState`.
- Response: `{ok: true, logoUrl: "/api/v1/_theme/logo?v=<hash>"}`.

#### `DELETE /api/v1/_theme/logo`

- Under `mutateState`: call `kv.Delete("theme/logo")` and `kv.Delete("theme/logo.ext")`; both can no-op if file already missing.
- Clear `state.UserLogoBytes`, `UserLogoExt`, `UserLogoHash`.
- Returns 204.

### Loading on app boot

In `dataentry/app.go` `New(ctx, ...)` (around the existing `loadUserPalette`
call): call `loadUserLogo()`, populate `UserLogoBytes`, `UserLogoExt`,
`UserLogoHash`. Failure to read (file present but unreadable) is a startup error
— same policy as `loadUserPalette` per RR-OA4A. Failure to read because the file
is missing is silent.

### Settings response shape

`APISettingsData` (`handlers_api.go:679`) gains:

```go
LogoUrl *string `json:"logoUrl,omitempty"`
```

`handleAPIGetSettings` builds it:

```go
if s.UserLogoHash != "" {
    u := "/api/v1/_theme/logo?v=" + s.UserLogoHash
    data.LogoUrl = &u
}
```

The SPA reads `data.logoUrl` from `getSettings()` on boot; the schema store
exposes it as a ref; Sidebar consumes it.

### Cache-busting

Content hash (`sha256.Sum256(bytes)[:6]` hex = 12 chars) goes in the query
string. New upload → new hash → new URL → browser fetches new bytes. No need to
wrestle with `If-Modified-Since` or `ETag` for this MVP — the URL itself is the
version.

### Frontend wiring

```ts
// frontend/src/stores/schema.ts (add)
const logoUrl = ref<string | null>(null)
function applyLogo(url: string | null) { logoUrl.value = url }
return { ..., logoUrl, applyLogo }

// frontend/src/api/theme.ts (new)
export async function uploadLogo(file: File): Promise<{ logoUrl: string }> { ... }
export async function removeLogo(): Promise<void> { ... }
```

Sidebar template:

```vue
<RouterLink to="/" class="logo">
  <img v-if="logoUrl" :src="logoUrl" :alt="appName" class="logo-img" />
  <span v-else>{{ appName }}</span>
</RouterLink>
```

`logoUrl` comes from `useSchemaStore().logoUrl` (or a new `useThemeStore` if we
want to keep schema clean — leaning toward putting it in `schema` since palette
state is already there).

### Settings UI

```vue
<section class="settings-section">
  <h2>Logo</h2>
  <div class="logo-preview">
    <img v-if="logoPreviewUrl" :src="logoPreviewUrl" alt="Logo preview" class="logo-preview-img" />
    <p v-else class="muted">No logo set — sidebar shows the app name as text.</p>
  </div>
  <div class="logo-actions">
    <input ref="fileInput" type="file"
           accept="image/png,image/jpeg,image/svg+xml,image/webp"
           hidden @change="handleFilePicked" />
    <button class="btn" @click="fileInput?.click()">Choose image</button>
    <button class="btn btn-primary" :disabled="!stagedFile || uploading"
            @click="handleUpload">Upload</button>
    <button v-if="logoUrl" class="btn btn-danger" @click="handleRemove">Remove</button>
  </div>
  <p class="muted-caption">PNG, JPEG, SVG, or WebP. Max 256 KiB.</p>
</section>
```

`logoPreviewUrl` is `URL.createObjectURL(stagedFile) || logoUrl` (i.e.
picked-but-not-uploaded preview, or current saved logo). On successful upload,
revoke the object URL and clear `stagedFile`.

**Alternatives considered:**

1. **Dedicated `GET /api/v1/_theme/logo/info` endpoint** to discover whether a logo is set. Rejected: extends `_settings` instead — fewer round-trips on app boot.
2. **Use `Put(empty)` instead of adding `Delete` to KV.** Rejected: then `GET` has to distinguish "exists but empty" from "doesn't exist" with extra logic, and the `KV` interface is already a leaky abstraction without `Delete`. PR 2 + PR 3 both want it too.
3. **Server-side SVG sanitization** (handroll or bluemonday). Rejected per the crit thread — `<img>` sandbox is the trust boundary; sanitization is fragile and redundant.
4. **PNG-only allowlist.** Rejected — SVG support is genuinely valuable for vector marks; the security analysis above carries it.
5. **Store the extension as part of the kv key (`theme/logo.png`).** Rejected: requires directory listing to find it after restart; sidecar `.ext` file is simpler.
6. **Compute hash from `Last-Modified`.** Rejected: less precise (filesystem mtimes have second granularity), and contents are tiny so SHA-256 is free.

**Files to modify:** see Research section above.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Input | Source | Validation |
|---|---|---|
| Multipart `logo` field | Logged-in user via PUT | `MaxBytesReader` at 257 KiB cap before parsing. Multipart parsed with `r.ParseMultipartForm(257<<10)`. |
| Logo bytes | Inside multipart | `http.DetectContentType(bytes[:512])` MUST match allowlist `{image/png, image/jpeg, image/svg+xml, image/webp}`. |
| Logo extension | Sidecar file `.rela/theme/logo.ext` | Read-back validated as one of `{"png","jpeg","svg","webp"}`. Anything else → treat as no logo and log a warning. |
| Filename in upload | Multipart header | Ignored. We always rename to `logo.<ext>` based on sniffed type. Never reflected in HTML or filenames. |

**Security-Sensitive Operations:**

- **SVG XSS:** browser `<img>` sandbox is the trust boundary. Mitigations layered on top:
  - We never inline SVG into the page (no `v-html`, no `dangerouslySetInnerHTML`-equivalent).
  - `Content-Type: image/svg+xml` plus `X-Content-Type-Options: nosniff` to ensure the browser treats the response as image, not HTML.
  - `Content-Security-Policy: sandbox` on the response so even direct navigation to the URL cannot run scripts.
- **Stored XSS via filename:** filenames are never reflected; `Content-Disposition` is not set on GET (we want inline rendering, not download).
- **Path traversal:** `state.KV` keys go through `RootedFS.resolve` (`internal/storage/rooted.go:85`) which is the path-validation barrier. Our keys are static literals (`theme/logo`, `theme/logo.ext`), so user input never reaches the path.
- **DoS via large upload:** 257 KiB `MaxBytesReader` is the hard cap. `ParseMultipartForm` enforces the same limit. No stream copy without the limit.
- **Image-decoder vulnerabilities:** we never decode the image server-side (no `image.Decode` call). The bytes are passed through to the client as-is; the decode happens in the browser, which is the same trust surface as any other web image.
- **CPU on hash:** SHA-256 over ≤256 KiB takes <1 ms on commodity hardware. Negligible.
- **Authn:** these endpoints inherit data-entry's existing auth (none today; localhost-only). No new attack surface beyond `_palette`.

**Error responses:**

- Always return JSON `{error: "<short category>", code: "<machine_code>"}`.
- Categories: `payload_too_large`, `unsupported_format`, `multipart_parse_failed`, `internal`. No bytes / file headers / stack traces in the response.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | How tested |
|---|---|
| 1. Settings shows Logo control | `frontend/src/views/SettingsView.test.ts` (Vitest + Vue Test Utils): mount with stubbed schema, assert presence of file input + buttons. |
| 2. Upload replaces sidebar text | Playwright e2e in `/e2e/`: upload PNG fixture, wait for toast, assert `Sidebar img.logo-img` exists. |
| 3. Remove reverts to text | Playwright e2e: with logo set, click Remove, assert `Sidebar` shows text and `GET /api/v1/_theme/logo` returns 404. |
| 4. Validation matrix | Go table-driven test in `internal/dataentry/handlers_api_logo_test.go`: rows for size cap, mime allowlist, malformed multipart. Each row checks status + body code + that on-disk state is unchanged. |
| 5. SVG `<script>` cannot execute | Playwright e2e with hostile SVG fixture: dialog event listener, network listener; assert neither fires. |
| 6. Sidebar layout | Visual e2e + scoped Vitest snapshot of `Sidebar.vue` with both logo states. Manual visual check during implementation. |

**Edge Cases:**

- Upload of 0-byte file → 400 (multipart provides empty body — sniffed as `application/octet-stream` → unsupported_format).
- Upload exactly at 256 KiB cap → accepted (boundary).
- Upload at 256 KiB + 1 byte → 413.
- Multiple concurrent PUTs from two browser tabs → `mutateState` serializes; last write wins. Hash query param ensures both tabs converge to the same logo on next settings fetch.
- Upload, then GET before settings cache refreshes → both work because `AppState` updates atomically inside `mutateState`.
- Page reload while logo is loading → the second `<img>` request hits the same hashed URL; browser short-circuits via cache.
- SVG with embedded base64 data URI for raster image → allowed (no external fetch, fine).
- SVG referencing external `xlink:href="https://..."` → browser sandbox refuses to load.
- PNG with EXIF metadata → passed through opaquely; browser handles.
- WebP → behaves identically to PNG for our purposes; sniffed by `http.DetectContentType` (returns `image/webp`).
- Removal when no logo is set → 204 (idempotent).
- App restart with `theme/logo` present but `theme/logo.ext` missing → treat as no logo, log a warning, leave both files alone (don't proactively delete).

**Negative Tests:**

- POST a 1×1 GIF → 400 unsupported_format.
- POST `text/plain` body claiming to be PNG → sniffed as `text/plain`, 400 unsupported_format.
- POST with no `logo` form field → 400 multipart_parse_failed (missing field).
- PATCH method → 405.
- DELETE before any upload → 204 (idempotent, not error).
- GET when no logo set → 404 with `{error: "not_found"}`.
- Multipart body claiming `Content-Length: 100MB` → `MaxBytesReader` cuts it off at 257 KiB, 413.

**Integration test approach:**

- Go integration test in `handlers_api_logo_test.go` driving full PUT → GET → DELETE → GET cycle against an `app` with in-memory `state.KV`. Asserts byte-for-byte preservation across PUT/GET, hash propagation into the URL, and 404 after DELETE.
- Vitest test for `frontend/src/api/theme.ts` (mocked fetch) confirming request shape (FormData with `logo` field) and response handling.
- Playwright e2e test in `/e2e/` for the user-facing flow (upload, see sidebar update, remove, see fallback). This is the highest-value test because it exercises the real browser SVG sandbox.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `state.KV` interface change ripples to other implementations | Low | Low | Only one impl (`FSKV`); change is additive. CI catches missing implementations via the `var _ KV = (*FSKV)(nil)` assertion. |
| SVG sandbox bypass via undisclosed browser bug | Low | High | Defense in depth: `nosniff` + `Content-Security-Policy: sandbox`. Document in CLAUDE.md that user-uploaded SVGs MUST go through `<img>` only. The hostile-SVG e2e test catches regressions. Worst-case escape route: drop SVG from allowlist as a hotfix. |
| Sidebar collapsed state hides image because the existing `.logo` class applies | n/a | n/a | Verified during research: that's the desired behavior — image hidden in collapsed mode same as text. |
| `Cache-Control: immutable` causes stale logo if URL hash collision | Very low | Low | SHA-256 collision probability is negligible; even on collision, a no-op upload (same bytes) is harmless. |
| 256 KiB cap too small for some users | Medium | Low | The sidebar logo is rendered ~28 px tall; 256 KiB is generous for that display size. If a real user complaint arrives we can bump the cap — the validation is one constant. |
| User uploads a logo via PUT, then system crashes before sidecar `.ext` write | Low | Low | Both writes happen inside `mutateState`; on restart, the missing `.ext` file is logged and the logo is treated as not-set (per Edge Cases). User retries; idempotent. |

**Effort:** **s** — already set on TKT-WN7O. Backend ~150 LOC + tests; frontend
~80 LOC + tests; one trivial interface change. Realistic estimate: half a
working day.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — add a "Custom logo" subsection under data-entry settings docs.
- [ ] CLI help text — N/A.
- [ ] CLAUDE.md — N/A for this PR specifically (the `<img>`-only-for-user-SVG note is a reasonable addition but can wait until PR 3 lands and we do a CLAUDE.md pass for the whole theme system).
- [ ] README.md — N/A.
- [x] API docs — document `/api/v1/_theme/logo` (GET / PUT / DELETE) alongside `_palette` docs.

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** <!-- List review-response IDs -->
