---
id: PLAN-E0TIRX
type: planning-checklist
title: 'Planning: Operator customisation hooks: serve + inject custom.css/custom.js, @layer cascade fix, isCustomElement'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN — items 1, 2, 3, 6 of the proposal:
1. Serve `custom.css` / `custom.js` from the project root at `/_custom/*`.
2. Inject `<link>` / `<script type="module">` into the SPA `index.html`, **plus
the `@layer` change** that makes the injection actually effective.
3. `isCustomElement` in the Vue compiler config (both `vite.config.ts` and
`vitest.config.ts`).
4. Docs page: three-tier contract + gotchas.

OUT — items 4, 5: the `rela-` class hooks and the `<rela-slot>` *consumer* on
the next-action component. Blocked: the next-action feature does not exist. Note
`<rela-slot>` needs **no** registration to be inert — items 1–3 are
independently useful, and an operator can already define the element from
`custom.js`.

OUT — a `window.rela` JS API (deliberately excluded by the proposal).

**Acceptance Criteria:** see AC1–AC6 on TKT-3DBK6I. Each maps to a test in the
Test Plan below.

## Research

**Research Doc:** N/A — the design was supplied as a written proposal; the open
technical questions were settled empirically instead (below), which is cheaper
and more conclusive than a survey.

**Existing Solutions / prior art in codebase:**

- **`os.OpenRoot` traversal-resistant serving** is an established idiom with
three call sites. Canonical: `openAppEntry`
(`internal/dataentry/apps.go:144-192`) — `os.OpenRoot(projectRoot)` →
`root.OpenRoot(subdir)` → `Open(rel)`, with `path.Clean("/"+entry)` +
`fs.ValidPath` pre-checks, a `maxAppFileBytes` (4 MiB) size cap, and errors that
never leak system paths. Also `openLocalScript`
(`internal/script/action.go:172-212`) and `loadScript`
(`internal/script/executor.go:251-288`). **Reuse this shape; do not invent
one.**
- **Uniform 404**: `apps_handler.go:168-172` collapses every `openAppEntry` error
into one `http.StatusNotFound`. Matches the proposal's "absent file → 404".
- **`project.Context.Root`** (`internal/project/context.go:26`) reached in-package
as `a.paths.Root` (`internal/dataentry/app.go:92`). Convention at
`context.go:18-21`: filesystem-only dirs like `apps/` are **not** modeled as
`Context` fields — use a package-level constant + `os.OpenRoot`. `custom.css`
follows that convention.
- **Config**: `dataentryconfig.AppConfig` (`internal/dataentryconfig/config.go:110-129`)
is the natural home for an operator knob; `PlantUMLServerURL` is the closest
precedent (operator opt-in for third-party content, deliberately not defaulted
on).

**Prior art that CONSTRAINS this work (important):**

`internal/dataentry/CLAUDE.md:158-161` — *"The bridge SDK is **served, not
injected**… No server-side HTML rewriting of the app's index."* There is no
`html.Render` call anywhere in `internal/`; `parseAppMeta` (`apps.go:243-289`)
parses **read-only** for `<meta>` and serves original bytes untouched.

That rule is scoped to **`apps/<id>/index.html`** (operator-authored app pages),
not to rela's own SPA shell, so it does not forbid this ticket — but this ticket
would introduce **the first server-side HTML rewrite in the codebase**, and the
docs must say why the two cases differ (we own the SPA shell; we do not own an
app's HTML, and rewriting the latter would break the app-CSP bright line).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**The cascade problem — MEASURED, and it inverts the proposal's premise.**

The proposal assumed the SPA has no `<link>` and that bundler-injected CSS means
an operator `<link>` in `<head>` wins. A real production build (`npm run build`)
plus a browser showed otherwise: `index.html` **does** contain a `<link
rel="stylesheet">`, and the build emits **19 CSS files** — one eager, 18
route-level chunks appended at runtime by Vite's `__vitePreload` via
`document.head.appendChild(link)`. Measured order after route navigation:
`index.css` → `custom.css` → `ListView.css`. Verified at the *rendering* level:
at equal specificity `getComputedStyle` resolved to the route chunk
(`operatorWins: false`). So operator CSS loses to every route view — a skin that
works on the dashboard silently dies on a list view.

**Chosen: `@layer` (option 2) + honest docs (option 4).** Validated empirically,
not assumed, in two steps:

1. `@layer` **survives Vue's scoped compiler**. Compiling an SFC with
`@vue/compiler-sfc` `compileStyle({scoped:true})` emits:
   ```
   @layer rela {
   .probe-target[data-v-testhash] { color: rgb(255, 0, 0); }
   }
   ```
Layer preserved, scoping still applied inside it, zero errors.
2. **Unlayered operator CSS wins** even in the hostile case — *lower* specificity
(0,1,0) against a layered rule (0,2,0) in a chunk appended *later*. Browser
result: `operatorWins: true`. This is exactly the case broken today.

**Layer scope decision (the ticket's open question): wrap ALL rela CSS.**

Census: 58 `<style scoped>` blocks, 5 unscoped blocks (`App.vue`, `Sidebar.vue`,
`Badge.vue`, `TagSelect.vue`, `RelationCards.vue`), 8 plain CSS files in
`src/styles/`, 67 `.vue` files total. No `@layer` exists anywhere in
`frontend/src/` — greenfield.

Partial layering is rejected: it leaves operator CSS beating some rela rules and
losing to others depending on which file a selector happens to live in, which
reintroduces exactly the route-dependent inconsistency this ticket exists to
remove. One layer, applied uniformly, is the only version with a statable rule.

**Mechanism — prefer a build-time wrap over editing 63 style blocks by hand.** A
Vite CSS post-plugin that wraps emitted CSS in `@layer rela { … }` covers all 19
outputs uniformly and cannot drift as new components are added. Hand-editing
every `<style>` block is rejected: 63 edit sites, silently incomplete the moment
someone adds a component. **This is the main implementation risk** (see Risks) —
if a clean plugin hook is not reachable, fall back to wrapping the 8
`src/styles/*.css` files plus a documented lint rule, and re-scope the ticket.

**SPIKE COMPLETED — risk 1 is RESOLVED, approach validated end-to-end.**

The `@layer` wrap plugin was built and run against a real production build
before this plan was presented. Result: **all 19 CSS assets wrapped**, build
clean, scoping preserved inside the layer.

```ts
// frontend/layerplugin.ts
export function relaLayer(): Plugin {
  return {
    name: 'rela-css-layer',
    enforce: 'post',
    generateBundle(_options, bundle) {
      for (const [name, chunk] of Object.entries(bundle)) {
        if (chunk.type === 'asset' && name.endsWith('.css')) {
          const src = String(chunk.source)
          if (src.trim() && !src.includes('@layer rela')) {
            chunk.source = `@layer rela {\n${src}\n}\n`
          }
        }
      }
    },
  }
}
```

Verified against the REAL built CSS in a browser — operator rule `.sidebar`
(0,1,0, unlayered) vs rela's actual `.sidebar[data-v-f40ef6f2]` (0,2,0,
layered):

| | computed background |
|---|---|
| before a later route chunk loads | `rgb(0, 255, 0)` (operator) |
| after `ListView-*.css` appended | `rgb(0, 255, 0)` (operator) |

`operatorWins: true` in both. The lower-specificity operator rule beats the
higher-specificity rela rule, and keeps winning after a later chunk loads —
precisely the case that fails today.

Two build-config frictions found by the spike (mechanical, but they WILL bite
the implementer):
- `tsconfig.node.json` `include` lists only `vite.config.ts`; the plugin file
must be added or `vue-tsc` fails with TS6307. (`vite.editor.config.ts` is also
absent from that list — pre-existing, unrelated, worth a glance.)
- The plugin must be `.ts`, not `.mjs`: an untyped `.mjs` import fails typecheck
with TS7016.

The spike was **reverted**; only the `vitest.config.ts` fix remains on the tree.
Implementation should re-apply it as reviewed code with the drift test below.

**Injection mechanism.** `spaHandler` (`router.go:640-658`) serves `index.html`
straight from the embedded FS via `http.FileServer`. Inject by intercepting only
the index response. Two sub-decisions:

- Compute the injected HTML **once at startup**, not per request. The shell is
embedded and immutable; the only variable is whether the two files exist. Per
request we need a cheap existence check (the proposal's "absent → no
injection"). Watching for file creation is out of scope — document that adding
`custom.css` needs a server restart, or re-stat per request if cheap.
- Rewriting bytes breaks `http.FileServer`'s `Content-Length`/`ETag`. Serve the
modified shell with `http.ServeContent` or write it directly with a correct
`Content-Length` rather than letting `FileServer` set headers for the original
bytes. **`golang.org/x/net/html` parse+render is NOT the right tool** — a full
round-trip normalizes the whole document (attribute quoting/order, entity
re-encoding) and would be the first `html.Render` in the codebase. A targeted
string insertion before `</head>` / `</body>` on our own embedded, known-shape
shell is smaller, lossless, and testable. (The proposal suggested x/net/html
because it's already a dependency; availability isn't a reason to use it.)

**Files to modify:**
- `internal/dataentry/custom.go` (new) — `openCustomAsset`, injection helper.
- `internal/dataentry/custom_handler.go` (new) — `/_custom/` route.
- `internal/dataentry/router.go` — register `/_custom/`; wrap `spaHandler`.
- `internal/dataentryconfig/config.go` — optional injection-disable flag.
- `frontend/vite.config.ts` — `isCustomElement` + the `@layer` wrap.
- `frontend/vitest.config.ts` — `isCustomElement` (**already applied**).
- `docs/` — new page; `internal/dataentry/CLAUDE.md` — note the SPA-shell
rewrite as a deliberate, scoped exception to the "served, not injected" rule.

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined (allowlist)
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- The request path under `/_custom/`. **Allowlist of exactly two names**
(`custom.css`, `custom.js`) — strictly tighter than `apps/`, which must accept
arbitrary entries. Anything else → 404. This makes traversal structurally
impossible before `os.OpenRoot` is even reached; `os.OpenRoot` remains as
defence-in-depth, matching `openLocalScript`'s "earlier rejection gives better
errors" comment.
- File **contents** are operator-authored and served verbatim. Not sanitized —
that is the stated trust model (the operator already controls metamodel, Lua,
ACL).

**Security-Sensitive Operations:**
- Project-root file read → `os.OpenRoot`, allowlisted names, size cap
(reuse `maxAppFileBytes`), reject directories, uniform 404 with no path in the
error (per `apps.go` idiom).
- Correct `Content-Type` (`text/css`, `text/javascript`) + `X-Content-Type-Options:
nosniff`. Note `/` currently has **no** CSP/nosniff/cache middleware
(`noCacheMiddleware` is `/api/`-only; `TestNewRouterStaticFilesNoCacheHeader`
pins the SPA having no `Cache-Control`) — so this handler sets its own headers,
per-handler, like `apps_handler.go:125-126` does.
- **No privilege escalation**: same-origin static files, no new capability. The
scripts run with the session cookie, but so does all SPA code already.
- **Not a secret**: per the root CLAUDE.md rule, the *existence* of `custom.css`
is config-level, not entity data — a 404-vs-200 distinction here leaks nothing
requiring concealment.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Negative test cases defined
- [x] Integration test approach defined

Conventions to follow (from `apps_test.go`): package-internal tests, stdlib
`testing` only, anonymous-struct tables + `t.Run`, `t.TempDir()`,
`newHandlerTestApp(t)`, `got/want` messages. Model traversal tests on
`TestOpenAppEntry_Traversal` (`apps_test.go:397`).

**Test Scenarios:**
- AC1 → `TestOpenCustomAsset`: present → bytes + content-type; absent → 404;
`TestCustomAsset_Traversal` over `{"../secret.txt", "../../etc/passwd",
"/etc/passwd", "sub/../../secret.txt", "", "custom.css/", "CUSTOM.CSS"}`; a
directory named `custom.css` → 404; oversize file → rejected.
- AC2 → `TestSPAInjection`: with neither file, served HTML is **byte-identical**
to the embedded shell (the strongest form of "no injection"); with only
`custom.css`, exactly one `<link>` and no `<script>`; with both, both tags, each
exactly once, in the right position; `Content-Length` matches the body.
- AC3 → the cascade guarantee. A Go test cannot evaluate CSS. Assert the
invariant that *makes* it true — that built CSS output is layer-wrapped — as a
build-output test (all emitted `.css` wrapped in `@layer rela`), plus a frontend
unit test that `compileStyle` preserves `@layer`. A real end-to-end cascade
check belongs in `e2e/` (Playwright, real browser) — record as follow-up if not
done here.
- AC4 → **must be a `.vue` SFC** mounting `<rela-slot>` and asserting no
`Failed to resolve component` warning. See the trap below.
- AC5 → existing `relaEditor.test.ts` (15 tests) must stay green.
- AC6 → docs review; verbatim disclaimer present.

**Edge Cases:**
- Empty (0-byte) `custom.css` → serve 200 empty, still inject (present ≠ non-empty).
- File created/deleted while the server runs (restart-required vs re-stat).
- Symlink from `custom.css` to outside the project → rejected by `os.OpenRoot`.
- Unicode/NUL in the request path → allowlist rejects before FS access.
- Shell lacking `</head>`/`</body>` → inject nothing rather than corrupt output.
- Concurrent requests during injection-cache population.

**Negative Tests:** every traversal string 404s (not 403/500); a request for
`/_custom/other.css` 404s; an unreadable file 404s without leaking the path.

**⚠ Test-methodology trap (cost a false negative already):** a probe using a
runtime **string `template`** showed the Vue warning persisting even with the
fix applied. Runtime-compiled templates never see build-time `compilerOptions`.
The AC4 regression test **must be a `.vue` SFC**; a string-template test fails
confusingly forever.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated

**Risks:**
1. **`@layer` wrap mechanism — RESOLVED by spike (was HIGH).** See the spike
result under Approach. A `generateBundle` post-plugin wraps all 19 CSS assets;
build clean, cascade verified in a browser. Residual risk is now only that a
future CSS asset escapes the wrap — *mitigation:* a build-output test asserting
every emitted `.css` starts with `@layer rela`.
2. **`@layer` changes existing precedence (MEDIUM).** Layering *all* rela CSS
alters how rela's own rules interact with anything unlayered — notably the 5
unscoped `<style>` blocks and any third-party CSS (EasyMDE/CodeMirror in the
editor bundle). *Mitigation:* full visual/e2e pass; watch `MarkdownEditor` and
`Badge`/`TagSelect` specifically.
3. **First server-side HTML rewrite in the codebase (MEDIUM).** Cuts against a
documented convention for `apps/`. *Mitigation:* targeted string insertion, not
a parse/render round-trip; document why the SPA shell differs from an app index.
4. **`Content-Length`/`ETag` mismatch (MEDIUM).** `http.FileServer` sets headers
from the original bytes. *Mitigation:* serve the modified shell explicitly;
assert `Content-Length` in tests.
5. **Browser support for `@layer` (LOW).** Baseline since 2022; rela already
targets modern evergreen browsers (ES modules, custom elements).
6. **Operator errors look like rela bugs (LOW, accepted).** Mitigated by docs
only — the "remove custom.js and retry before filing a bug" line.

**Effort:** L. Unchanged. The `@layer` work is larger than the proposal assumed;
items 1/3 are small.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] New `docs/` page — three-tier table, hooks, worked example, verbatim
stability disclaimer, both gotchas (DOM injected in `#app` is destroyed on
re-render; no route-change events for outside JS).
- [x] `docs/data-entry.md` — cross-reference.
- [x] `internal/dataentry/CLAUDE.md` — the SPA-shell rewrite as a deliberate,
narrowly-scoped exception to "served, not injected"; the `@layer rela`
convention so new CSS stays inside the layer.
- [x] `frontend/CLAUDE.md` — the `@layer` convention + the SFC-not-string-template
testing trap.
- [x] ~~`docs/metamodel.md`, `docs/cli-reference.md`, `README.md`~~ (N/A: no metamodel, CLI, or project-level surface changes)

## Design Review Resolutions (all 11 findings folded in)

**DECISION 1 — `tokens.css` is OUT of the layer.** Its purpose is a stable
contract so iframe-contained apps look similar to the rest of rela. It is served
into TWO different cascade environments: in the SPA among rela's own CSS, and in
an app iframe as `_rela.css` alongside the app author's CSS and *nothing else of
rela's*. Layering is only meaningful relative to other layered CSS — inside an
app iframe there is no other rela CSS, so `@layer rela { :root {...} }` would
not order tokens against anything, it would merely demote them beneath every
unlayered rule the app author writes. The contract would silently weaken in
exactly the environment it exists to serve. Unlayered keeps one file behaving
identically in both places. This dissolves RR-XOTMPN; add an assertion that
NEITHER copy contains `@layer` so nobody wraps it by accident.

**DECISION 2 — ONE FLAT `@layer rela`. Sub-layers rejected.** No operator-facing
benefit exists: operator CSS is *unlayered*, so it already beats every layer
regardless of count or order. Sub-layers would only order rela's CSS against
itself — an internal-precedence problem we do not have. All three motivating
cases dissolve: tokens are now out of the layer (decision 1); vendor CSS
(RR-F371WB) is a docs question, not a layer-structure one, since a `rela.vendor`
sub-layer is still beaten by unlayered operator CSS; components are not
distinguishable from base today. Cost is real and standing: every new stylesheet
would need a sub-layer assignment forever with no rule to decide it by. The
"asymmetric migration" argument for pre-emptive sub-layers was overstated — flat
to sub-layers is only breaking for skins depending on INTER-layer ordering, and
since operator CSS outranks all layers, almost nothing can depend on that.

Keep the one part of RR-SZ1JY2 that pays for itself: emit a bare `@layer rela;`
declaration into the eager `index.css` before any content, pinning layer
position at first parse. One line; closes a real order-dependence hole given 18
runtime-appended chunks.

**Resolutions for the remaining findings:**
- RR-8R8U0B (`!important` inversion, CRITICAL) — real, permanent, cannot be
designed away. AC6 docs MUST state it. Verified in-browser: rela's layered
`!important` beats an operator's later unlayered `!important`.
- RR-ON8XUE (AC3 unverifiable, CRITICAL) — write the Playwright e2e test. NOT a
follow-up; it is the only test that verifies AC3 as written.
- RR-F371WB (vendor CSS) — vendor CSS stays IN the flat layer; AC6 documents
that non-rela-authored selectors (`.CodeMirror`, `.editor-toolbar`, `.fa-*`) are
tier 3, outside the stability promise, and break on any editor swap.
- RR-NK0VW9 (dev/prod divergence) — ACCEPTED and documented in
`frontend/CLAUDE.md`. Also record that `build:e2e` is a real `vite build`, so
e2e does see the layer — nobody should "optimize" e2e onto the dev server.
- RR-01CKP9 (injection caching) — precompute FOUR variants at startup
(none/css/js/both), re-stat per request to select. Kills the cache-population
race outright and removes the restart caveat.
- RR-N7IX9Z (caching) — `Cache-Control: no-cache` on `/_custom/*`; assert in test.
- RR-6OFNPA (exception rationale) — document the SECURITY-boundary argument with
the explicit CSP trip-wire, not the ownership argument.
- RR-61F77L (two trust models) — docs must state `custom.js` is fully-trusted and
unconfined, whereas `apps/` is sandboxed; no reader should conflate them.
- RR-6J7SO7 (empty CSS) — wrap unconditionally so "every emitted .css is layered"
is literally true.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-XOTMPN, RR-8R8U0B, RR-ON8XUE (critical);
RR-F371WB, RR-NK0VW9, RR-01CKP9, RR-N7IX9Z, RR-6OFNPA (significant); RR-61F77L,
RR-6J7SO7, RR-SZ1JY2 (minor). All resolved above.
