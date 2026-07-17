---
id: TKT-5F9V56
type: ticket
title: Expose a markdown editor to custom apps via a <rela-editor> custom element (_rela-editor.js)
kind: enhancement
priority: medium
effort: m
status: ready
---

Custom apps currently have to build their own `<textarea>` for editing markdown
(the "Today" app's notes box is a plain textarea). There's no way to give a
plugin rela's real markdown editing experience, and no high-level seam that
would let us swap the underlying editor later without breaking plugins.

## Goal

A **high-level, minimal, swappable** markdown editor API for custom apps. The
plugin-facing contract must be small enough that the underlying editor (today
EasyMDE/CodeMirror 5) can be replaced later (e.g. CodeMirror 6) with **zero
plugin changes**.

## Design (converged with maintainer)

Ship a **Custom Element** `<rela-editor>` from a new reserved per-app asset
`_rela-editor.js` (separate from the tiny bridge `_rela.js` so only apps that
opt in pay the ~150KB editor bundle).

- **Custom Elements, NOT shadow DOM.** EasyMDE is built on CodeMirror 5, which has known shadow-DOM focus/selection/measurement bugs. The element renders EasyMDE into its own **light DOM** so CM5 behaves; encapsulation is by *convention* (documented narrow API; internals unsupported). The iframe sandbox already isolates apps from each other, so a plugin poking its own editor only hurts itself. Cleanly upgrades to enforced shadow-DOM encapsulation if/when the SPA moves to CM6 — **without changing the plugin-facing API**.

- **Public contract (the swap seam) — this and nothing else:**
  - property `value` (get/set markdown; whitespace-exact)
  - attribute `placeholder` (plain text, attribute-safe)
  - attribute `readonly` (boolean)
  - native `input` event (per keystroke) and `change` event (on blur/commit), dispatched on the element
  - `focus()`
  - `connectedCallback` builds EasyMDE; `disconnectedCallback` destroys it + removes listeners (leak-safe even if the plugin removes the node)

- **Initial value via the JS property only** (`ed.value = '...'`). No `value` attribute and no slotted text content — markdown is multiline and whitespace-sensitive; HTML attribute/text-content normalization would mangle it.

- **Pure text editor.** Markdown in, markdown out. No entity binding, no autosave — the plugin wires `change` -> `rela.update()` itself.

## Toolbar icons / font — RESOLVED

Keep EasyMDE's real Font Awesome 4.7 toolbar, and **serve the FA webfont under
the app's own base** as a reserved asset (e.g. `_rela-editor.woff2`). The editor
bundle's `@font-face` points at that app-relative URL, which the EXISTING
`font-src <base>` already permits — **no CSP change, no bright-line exception,
no hash coupling.**

Rationale for rejecting the alternatives:
- *Widen `font-src` to `<base> /assets/`* to reuse the SPA's FA font: syntactically fine and **safe today** (CSP normalizes then prefix-matches, so no traversal; `/assets/` holds only public static build output, no API/entity data). Rejected only to preserve the "apps touch ONLY their own path" bright line (keeps the app CSP cheap to audit forever; avoids a future asset under `/assets/` silently inheriting app font-load permission) AND because the SPA's FA file is hash-named/rebuilt each `npm run build` (would need a manifest lookup). NOTE for the implementer: the security fear about traversal was investigated and is NOT real — the reason is maintainability/bright-line only.
- *Inline SVG icons*: viable, but the FA-under-base option keeps the toolbar identical to the SPA with comparable effort now that the font is served same-path.

The FA webfont is already in the build
(`static/v2/assets/fontawesome-webfont-<hash>.woff2`, ~77KB woff2) but only as a
hash-named, untracked build artifact, so it is not directly reusable. The
editor's own build must emit a **stable-named** woff2 to embed + serve at the
reserved path.

## Implementation sketch

- New frontend Vite **lib/IIFE build** (separate config) with entry `src/app-editor/relaEditor.ts` that bundles EasyMDE + CodeMirror 5 + EasyMDE CSS into a self-contained IIFE which `customElements.define('rela-editor', ...)`. Drops the SPA's entity-picker button + backtick-autocomplete (those need the schema store; the app editor is pure text). The bundled CSS `@font-face` for FA points at the app-relative reserved font path.
- Emit a stable-named FA `woff2` from this build; embed both the JS bundle and the woff2 into the Go binary via `//go:embed` (same pattern as `apps_tokens.css`), each with a drift/sync test.
- `apps.go`: add reserved constants `appEditorEntry = "_rela-editor.js"` and `appEditorFontEntry = "_rela-editor.woff2"`; `apps_handler.go`: serve the JS as `text/javascript` and the font as `font/woff2`, both under the app base (existing `script-src <base>` / `font-src <base>` already permit). Reserved-entry shadowing (apps can't serve `_`-prefixed files) already covers both.
- App author usage:
  ```html
  <script src="_rela.js"></script>
  <script src="_rela-editor.js"></script>
  <link rel="stylesheet" href="_rela.css" />
  ...
  <rela-editor id="ed"></rela-editor>
  <script>
    window.addEventListener('rela:ready', async () => {
      const note = await rela.get({type:'daily-note', id});
      const ed = document.getElementById('ed');
      ed.value = note.content || '';
      ed.addEventListener('change', () => rela.update({type:'daily-note', id, patch:{content: ed.value}}));
    });
  </script>
  ```
- Docs: extend the custom-apps doc + `internal/dataentry/CLAUDE.md` (the `_rela-editor.js` / `_rela-editor.woff2` reserved assets and the `<rela-editor>` contract; value-API-is-the-contract / internals-unsupported rule, mirroring the `_rela.css` "tokens are the contract" note; and the explicit decision to serve the font under `<base>` rather than widening `font-src`).
- Tests: served content-types + reserved-entry shadowing for both new assets; bundle + font drift guards; (frontend) a smoke test that `<rela-editor>` mounts, round-trips `value` whitespace-exact, and fires `change`.

## Forward-compat note

Because the contract is
`value`/`input`/`change`/`focus`/`placeholder`/`readonly` only, swapping EasyMDE
for CM6 (and tightening to a closed shadow root) is invisible to plugins. If a
future change DOES alter the element contract, bump `currentBridgeVersion` and
gate per declared app version (same seam as the bridge SDK).
