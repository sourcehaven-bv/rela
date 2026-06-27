# dataentry — rules for new code

The data-entry web app (Go API + Vue 3 SPA in `frontend/`). Two rule sets
apply here: the write-validation policy, and the `_actions` affordance
contract.

## Validation policy for write APIs

rela's storage is permissive: markdown + YAML frontmatter, edited freely by
external tools alongside the API. Philosophy: **tolerate temporarily invalid
data**; the `analyze_*` tools surface inconsistencies the storage layer
doesn't reject.

Write-time checks split into three classes (DEC-HWZHA):

| Class | When | HTTP |
|---|---|---|
| **Hard 400 — malformed wire format** | Request structure broken, detectable without the metamodel | 400 |
| **Hard 422 — structural impossibility** | Storage layer literally cannot persist this | 422 |
| **Write-with-warnings** | Soft conditions: target type mismatch, missing target, unknown/required-unset/mistyped meta keys | 200 |

The 200 path performs the write and returns warnings `{code, path, detail}`
in the body — `code` matches the corresponding `analyze_*` finding code so
UIs de-duplicate against analyze runs.

**Resist drift toward hard rejection on soft conditions.** Before adding a
422 on a write path, ask: *could a hand-editor produce this state in a
markdown file?* If yes, it's soft — warn, don't reject. JSON:API-style
"validate-then-422" assumes wire and storage share a closed schema; rela's
storage is intentionally more permissive.

## Action affordances (`_actions`)

Every entity and list response carries `_actions: map[string]bool`. The SPA
reads it to decide which write controls to render. The map is a **UI hint** —
the server re-authorizes every write.

Rules for new write code here:

- **Route every `acl.WriteRequest{Op:...}` through `translateVerb`** in
  `affordances.go`. A grep test (`lint_test.go`) enforces it: no other file
  in this package may construct the literal. The shared constructor is the
  structural guarantee that the affordance map and the actual write resolve
  to the same ACL request.
- **Don't trust `_actions` for authorization.** The write endpoint must
  re-authorize. `affordances_contract_test.go` pins the invariant: every
  `_actions[v] == false` ⇒ 403 on the write, every `true` ⇒ 2xx.
- **New verbs require coordinated changes:** add an `acl.Op` constant, a
  `translateVerb` case, and update `docs/data-entry/api-reference.md`. Old
  SPAs ignore unknown keys; removing/renaming a verb is a major API bump.
- **Phase 1 verbs:** `create` (per-collection), `update`/`delete`/`rename`
  (per-item). `transition:*` and `relation:*` are deferred until ACL gains
  Op variants or extension fields.

Rules for new write affordances in the Vue SPA (`frontend/`):

- **Gate every entity-CRUD button** on `entity._actions?.[verb] !== false`
  (or `listResponse._actions?.create !== false` for collection verbs). False
  → hide; anything else (true/undefined/absent) → render. Absent is the
  defensive-render fallback for non-data-entry callers; the server still 403s.
- **No `useACL()` composable or client-side ACL evaluator.** TKT-AWM6L's
  wont-fix rejected this. The SPA reads booleans the server computed — no
  computation, merging, or prediction.
- **Adding a write affordance** requires (a) a backend `translateVerb` entry
  plus a `perItemVerbs`/`perCollectionVerbs` update, and (b) the inline
  `v-if` on the component. No ESLint enforcement; code review catches drift.

## Custom apps (`apps/<id>/` + `_apps/{id}/...` + the bridge)

User-authored apps served in a sandboxed iframe. An app is a **folder**
`apps/<id>/` with an `index.html` (plus sibling assets). Rules for new code:

- **Apps are folder-discovered on disk**, served per-entry via `os.OpenRoot`
  (see `apps.go`). No `apps:` config — `scanApps` lists `apps/<dir>/` that have
  an `index.html`; id = folder name; metadata from `<meta name="rela-app:*">` in
  index.html. Unpublish by renaming the folder or removing index.html. Never
  store apps in the entity store — filesystem in every backend, like
  `actions/`/`templates/`. The scan + per-entry reads are per-request (no
  watcher wiring); fine for a handful of apps.
- **Apps MUST declare `<meta name="rela-app:bridge-version" content="N">`.**
  `currentBridgeVersion` (apps.go) is the contract this server serves;
  `validateBridgeVersion` rejects a missing/unparseable version and one NEWER
  than the server (scanApps drops it from the listing; handleV1App 422s the
  index serve). This is the forward-compat seam: on a breaking bridge change,
  bump `currentBridgeVersion` and add a per-version path (e.g. a version-matched
  `_rela.js`) keyed on the app's declared version — don't break old apps. Older
  (<= current) versions stay allowed.
- **The app loads same-origin from `/api/v1/_apps/<id>/`** (iframe `src=`, not
  `srcdoc`) so its sibling files resolve. It is therefore same-origin with the
  API — **the path-scoped CSP header is the whole boundary**, not origin-`null`.
- **The CSP is a path-scoped HEADER, not a `<meta>`** (`appCSP(base)` in
  `apps_handler.go`). Every resource directive is scoped to the app's own
  subpath (`script-src /api/v1/_apps/<id>/ …`), NOT `'self'` (which would
  include `/api/`, letting `<img src=/api/v1/tickets/x>` exfiltrate).
  `connect-src 'none'` blocks the app's own fetch/XHR/WS, so the
  `MessageChannel` bridge (not a network request) is the only path to the API.
  `form-action 'none'` + sandbox block form/nav exfil. CSP correctness is now
  the entire security boundary — keep the path-scoping exact and tested. Do NOT
  reintroduce a `<meta>` CSP or add an author-controlled egress allow-list
  (self-defeating).
- **The bridge SDK is served, not injected.** `appSDKSource()` is served at the
  reserved `/_apps/<id>/_rela.js`; apps include `<script src="_rela.js">`. The
  app cannot shadow `_rela.js` or serve any `_`-prefixed entry from its files.
  No server-side HTML rewriting of the app's index.
- **Readiness is replayable; prefer `rela.ready` / `rela.whenReady(cb)` over the
  `rela:ready` event.** The handshake can complete before an app's inline code
  runs (e.g. a large `<script src="_rela-editor.js">` between `_rela.js` and the
  app's listener delays it past the port arrival) — an `addEventListener('rela:ready')`
  added that late never fires. The SDK resolves a `rela.ready` Promise on the
  handshake (and `rela.whenReady(cb)`), which is not-missable. The event is kept
  for back-compat only. `TestAppSDKReadiness` pins this. Don't add `ready`/
  `whenReady`/`isReady` to `appSDKMethods` — they're local SDK helpers, not host
  calls.
- **Optional markdown editor: `<rela-editor>` from the reserved
  `/_apps/<id>/_rela-editor.js`** (`appEditorSource()`), with its glyph webfont
  at `/_apps/<id>/_rela-editor.woff2` (`appEditorFontSource()`, served with
  `Access-Control-Allow-Origin: *` because the sandboxed iframe is null-origin so
  the `@font-face` fetch is cross-origin). Both are embedded from
  `app_editor_dist/` — a **build artifact** produced by `frontend/vite.editor.config.ts`
  (`npm run build` runs it), gitignored like `static/v2`, with a committed
  `.gitkeep` so the glob embed compiles on a clean checkout;
  `TestAppEditorBundleEmbedded` skips when unbuilt and asserts the contract when
  built. Separate from `_rela.js` so only apps that opt in pay the ~370KB bundle.
  **The element's public contract is the swap seam — keep it minimal**: property
  `value` (whitespace-exact), attributes `placeholder`/`readonly`, native
  `input`/`change` events, `focus()`. Everything else (that it's EasyMDE/
  CodeMirror, the toolbar, the generated DOM) is unsupported, so the editor can
  be swapped later without breaking apps. Light DOM, not shadow DOM (CM5 misbehaves
  in a shadow root); upgrades to enforced shadow encapsulation if the SPA moves
  to CM6, without changing the contract. Programmatic `.value` sets are silent
  (no `input`), matching a native `<textarea>`.
- **Optional styling is served at the reserved `/_apps/<id>/_rela.css`**
  (`appCSSSource()` = theme tokens + the atomic `.btn`/`.input`/`.card`). The
  tokens are embedded from `apps_tokens.css`, a **byte-identical copy** of
  `frontend/src/styles/tokens.css` (the SPA's source of truth) —
  `TestAppTokensCSSInSyncWithFrontend` fails on drift; re-copy, don't hand-edit.
  Keep `_rela.css` to tokens + *atomic, pure-presentation* controls; component-
  shaped classes (tables, selects, modals) stay out (they'd become an
  unmaintainable frozen contract). Theme follows the host: the SDK toggles
  `dark` on the app's `<html>` from the `rela:theme`/handshake messages, using
  the same `:root.dark` selector as the SPA.
- **No app-specific ACL.** An app inherits the user's permissions: every
  `/api/v1/*` call runs under the user's session (reads → `readGate`, writes →
  `entitymanager`). Do **not** add an `OpRunApp`/`AppSubject` — it would break
  `--read-only` mode (`ReadOnlyACL` denies all `WriteRequest`s) and panic on the
  sealed-`Subject` switch. The app *is* the user. (The bridge is the seam where
  a future per-app *restriction* — e.g. read-only — would be enforced.)
- **The host bridge is a closed method allow-list, never a URL proxy.**
  `frontend/src/bridge/relaBridge.ts` maps each allowed method to one existing
  api-client call. Adding a capability = adding a named method, never a generic
  "fetch this path". Keep `appSDKMethods` (Go, in `apps_sdk.go`) in sync with
  `BRIDGE_METHODS` (the dispatcher).
