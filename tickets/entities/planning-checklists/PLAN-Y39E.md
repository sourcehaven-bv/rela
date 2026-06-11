---
id: PLAN-Y39E
type: planning-checklist
title: 'Planning: Bundle Font Awesome locally so EasyMDE doesn''t fetch it from a CDN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** EasyMDE's default behavior is to inject a `<link>` tag pointing at
`https://maxcdn.bootstrapcdn.com/font-awesome/latest/css/font-awesome.min.css`
at runtime (verified in `node_modules/easymde/dist/easymde.min.js`, gated by the
`autoDownloadFontAwesome` option which defaults to true). The default toolbar
buttons configured in `frontend/src/components/forms/MarkdownEditor.vue:71-90`
(bold, italic, heading, link, code, quote, preview, side-by-side, fullscreen,
guide) all rely on the FA 4.7 `fa fa-*` glyph classes that ship in that CSS.
Without the CDN load, those buttons render as blank squares.

**Scope:**

IN scope:
- Suppress EasyMDE's auto-injected CDN `<link>` (pass `autoDownloadFontAwesome: false`).
- Bundle Font Awesome 4.7 locally as an npm dependency and import its CSS so the EasyMDE toolbar glyphs continue to render.
- Verify the only `fa` class consumer is EasyMDE (the entity-ref button already uses inline SVG, `MarkdownEditor.vue:34-41`).
- Add an e2e assertion that no request is made to `maxcdn.bootstrapcdn.com` when the markdown editor mounts.

OUT of scope:
- Upgrading to FontAwesome 6/7 (`@fortawesome/fontawesome-free`). EasyMDE's bundled toolbar uses FA 4.7 class names (`fa fa-bold`); FA 6+ split classes (`fas fa-bold`) and would require monkey-patching toolbar buttons or maintaining CSS aliases — out of scope.
- The `cdn.jsdelivr.net/codemirror.spell-checker/...` dictionary URLs (only fetched when `spellChecker: true`; we set `false` in `MarkdownEditor.vue:56`). Re-noted in the ticket body as a future concern.
- Tree-shaking unused FA glyphs — `font-awesome@4.7.0` is ~30 KB gzipped CSS + the font files; the win isn't worth the build complexity for this size.

**Acceptance Criteria:**

1. **AC1 — no CDN request:** When a markdown editor mounts in the data-entry SPA, the browser makes zero requests to `maxcdn.bootstrapcdn.com`. **Test:** Playwright e2e test asserts on `page.on('request')` that no URL matching `/maxcdn\.bootstrapcdn\.com/` is requested during the lifetime of a form page with a markdown field.
2. **AC2 — icons render:** All default EasyMDE toolbar buttons (bold, italic, heading, unordered-list, ordered-list, link, code, quote, preview, side-by-side, fullscreen, guide) render their icons with the same visual appearance as today. **Test:** Existing `e2e/tests/markdown-editor.spec.ts` continues to pass; a new assertion confirms the bold button's computed font-family contains `FontAwesome`.
3. **AC3 — bundled origin:** Font Awesome assets are loaded from the SPA's own origin (i.e., `/assets/...` paths emitted by Vite, embedded into the Go binary). **Test:** Same network-trace assertion verifies all stylesheet/font requests are same-origin.
4. **AC4 — offline:** With `rela-server` running and the host's external network blocked (or simply running on loopback only), the toolbar icons still render. **Test:** Manual verification — start the server, open Chrome DevTools, set throttling to "Offline" after the SPA has loaded the SPA bundle and `index.html`, then open a form; toolbar icons remain.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **`font-awesome@4.7.0`** (npm) — the original FA 4.7 package, exactly what EasyMDE was built against. Class names `fa fa-bold` etc. match EasyMDE's hardcoded toolbar markup verbatim. Last published 2016 but stable. Chosen because it is a 1:1 replacement for the CDN URL EasyMDE injects.
- **`@fortawesome/fontawesome-free@7.x`** — modern, but uses `fas`/`far`/`fab` style prefixes. Would require either patching EasyMDE's `bindings.js` toolbar defaults (fragile across upgrades) or shipping a CSS shim mapping `.fa.fa-bold` → `.fas.fa-bold`. Rejected as more complexity than the bug warrants.
- **Inline SVGs per button** — possible, but means re-implementing every toolbar button via `ToolbarIcon` objects with custom `icon` SVGs (as we already do for the entity-ref button at `MarkdownEditor.vue:42-50`). Rejected: ~10 buttons × hand-copied SVG = noisy code, and EasyMDE's default toolbar behavior (state classes, active styling) is already wired to the FA class names.

**Similar patterns in codebase:**

- `frontend/src/main.ts:5-8` already imports `@fontsource/open-sans` CSS the same way we'll import font-awesome CSS — npm dep, `import '<pkg>/<file>.css'` in `main.ts`, Vite bundles and rewrites font URLs to `/assets/*` paths.
- `internal/dataentry/static.go:7` embeds the full Vite build output (`//go:embed all:static/*`) — no Go-side change needed; once Vite emits the FA woff/ttf files into `static/v2/assets/`, the embed picks them up.

**Reference implementation:** EasyMDE's own README documents
`autoDownloadFontAwesome: false` for exactly this case.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. Add `font-awesome@^4.7.0` to `frontend/package.json` dependencies.
2. In `frontend/src/components/forms/MarkdownEditor.vue` (or `main.ts` — leaning toward the component, since FA is conceptually owned by the editor and not the global app):
   - Add `import 'font-awesome/css/font-awesome.min.css'` next to the existing `import 'easymde/dist/easymde.min.css'`.
   - Pass `autoDownloadFontAwesome: false` in the `new EasyMDE({...})` options object.
3. Vite rewrites the FA CSS's `@font-face url('../fonts/...')` references to hashed `/assets/*.woff2` paths automatically during build — no config changes needed.
4. The build output goes to `internal/dataentry/static/v2/assets/` which is already covered by `//go:embed all:static/*`.

**Files to modify:**

- `frontend/package.json` — add `font-awesome` dependency.
- `frontend/package-lock.json` — auto-updated by `npm install`.
- `frontend/src/components/forms/MarkdownEditor.vue` — add CSS import, add `autoDownloadFontAwesome: false`.
- `e2e/tests/markdown-editor.spec.ts` (or a new `markdown-editor-bundling.spec.ts`) — add request-trace assertion.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** No new user input. The change removes an
uncontrolled external resource load, which is a security improvement — it
eliminates a CDN-supplied-CSS injection vector (compromised maxcdn could ship
malicious `content: url(...)` or `@import` directives executing in the user's
session origin).

**Security-Sensitive Operations:** No new crypto/auth/file-access surfaces.
Removing the external `<link>` slightly tightens CSP posture (a future strict
CSP policy no longer needs `style-src https://maxcdn.bootstrapcdn.com`).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1 → Playwright test installs `page.on('request')` handler before navigation; navigates to a form route with a markdown editor; asserts no captured URL matches `maxcdn.bootstrapcdn.com`. This is the load-bearing test — if EasyMDE's `autoDownloadFontAwesome` flag is ever flipped back to true, this fails.
- AC2 → Existing markdown editor e2e (`e2e/tests/markdown-editor.spec.ts`, `e2e/tests/markdown-editor-entity-ref.spec.ts`) continues to pass without changes. Plus: a focused check that `getComputedStyle(boldButton, '::before').fontFamily` contains "FontAwesome" (proves the icon font loaded, not just that the button exists).
- AC3 → Part of AC1's request trace: assert each captured stylesheet/font request URL has the same origin as `page.url()`.
- AC4 → Manual verification documented in IMPL checklist; recipe: build, run, open form, in DevTools Network panel disable cache + filter out the SPA bundle, reload the form, confirm no external requests fire.

**Edge Cases:**

- **EasyMDE upgrade** — a future EasyMDE version might rename `autoDownloadFontAwesome`, drop FA entirely, or change the toolbar class names. Mitigation: the e2e network assertion will catch a regression where FA stops being suppressed; the icon-visible assertion will catch a regression where icons disappear.
- **Browser font caching** — first load fetches the WOFF2 file; if the response is incorrectly cached as `Cache-Control: no-store`, every page reload re-fetches. Vite's default asset handling uses content-hashed filenames with long-term cache headers, so this is automatic.
- **Build artifact size** — FA 4.7 CSS + fonts add ~75 KB to the embedded static bundle. Acceptable; the Go binary is already several megabytes and this is one-time, not per-request.

**Negative Tests:**

- N/A — this is a build-time inclusion + a constructor option, not a runtime code path with branching. The only failure mode is "icons don't show" which is covered by AC2.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **R1 — FA 4.7 package staleness:** `font-awesome@4.7.0` has not been updated since 2016. Mitigation: it is a CSS+font asset library, no JS, no security surface; `npm audit` reports zero vulnerabilities for it.
- **R2 — Icon rendering subtly different from CDN version:** the CDN URL is `latest`, which today resolves to FA 4.7.0; pinning to `4.7.0` in npm gives the same content. Visual diff should be nil. Mitigation: AC2's existing e2e covers the user-visible behavior; a manual visual diff during implementation will confirm.
- **R3 — Vite font-emit path changes:** Vite hashes asset paths; if the FA `@font-face` CSS uses a path Vite can't resolve, fonts 404. Mitigation: verify during dev (`npm run dev`) and in the built bundle that the WOFF/WOFF2 files end up under `static/v2/assets/` and are referenced by hashed URLs.

**Effort:** s (1-2 hours: one dep add, two-line code change, one e2e test,
manual verification).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed. The fix is transparent to users (icons render identically) and to operators (binary is now genuinely self-contained, which is what the docs already imply).
- ~~User guide / reference docs~~ (N/A: no user-visible behavior change)
- ~~CLI help text~~ (N/A: no CLI changes)
- ~~CLAUDE.md~~ (N/A: no new pattern beyond what `main.ts`'s `@fontsource/open-sans` import already shows)
- ~~README.md~~ (N/A)
- ~~API docs~~ (N/A)

A short comment in `MarkdownEditor.vue` near the `autoDownloadFontAwesome:
false` line will explain the linkage so a future reader understands why both the
CSS import and the option exist.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small, low-risk single-component change — one CSS import plus one EasyMDE option, guarded by a `satisfies` typecheck and an e2e regression test. No new architecture or cross-subsystem surface to review.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: design review skipped per above)

**Design Review Findings:** *(N/A — design review skipped for this small, well-scoped change)*
