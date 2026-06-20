---
id: TKT-F5FDEQ
type: ticket
title: Optional rela design tokens + base controls CSS for custom apps (_rela.css)
kind: enhancement
priority: low
effort: m
status: backlog
---

Follow-up to TKT-9AW5RF (custom folder apps). Let an app opt into rela's look so
it renders consistently with the SPA, without coupling to component internals.

## Decided scope (tokens + atomic controls, opt-in)

Serve a small **`_rela.css`** at the app's own reserved path (like `_rela.js`).
An app opts in with `<link rel="stylesheet" href="_rela.css">`. Contents:

- **All value tokens** as CSS custom properties on `:root` — colors
(`--text-color`, `--bg-color`, `--card-bg`, `--border-color`, `--accent-color`,
`--error/success/warning/info-color`, the `--badge-*` set), fonts, spacing
scale, radii, shadows. Plus a `:root.rela-dark` block mirroring the SPA's
existing `:root.dark` overrides.
- **Three atomic, frozen-contract controls**: `.btn` / `.btn-primary` (already
exist in App.vue — reuse), `.input` (bare text-input border/padding — newly
authored for apps), `.card`. These are pure presentation with one obvious HTML
shape.
- **Nothing component-shaped.** No selects, tables, modals, pickers, badges-with-
logic, forms — those are Vue components/behavior, and a CSS-only version gives a
worse-than-nothing half-native result. Apps build those from the tokens.

The line, for future additions: publish it only if it's pure presentation with
one obvious HTML shape AND we'd freeze its appearance as a public contract.
Tokens are safe forever (renaming --text-color would break the SPA anyway);
components past the atomic three are not.

## Theme (dark mode)

Injecting the color *tokens* makes theming nearly free: `_rela.css` defines the
`:root` (light) and `:root.rela-dark` (dark) blocks, and the app's own CSS uses
`var(--…)`. Dark mode is then just toggling `rela-dark` on the app's `<html>`.
The SDK/bridge sets that class from the host's current theme (the SPA toggles
`.dark` on its own `<html>` via stores/ui.ts and `:root.dark` in App.vue) — a
small bridge message or a value passed at handshake. No per-property syncing.

## Security / serving (fits the existing model)

- Serve `_rela.css` from the binary at the reserved per-app path
`/api/v1/_apps/<id>/_rela.css` (reserved like `_rela.js`; apps can't shadow
`_`-prefixed entries). Content-Type `text/css`.
- **CSP carve-out:** the app's path-scoped `style-src /api/v1/_apps/<id>/` already
covers it (the CSS is served from the app's own path), so NO new CSP origin is
needed — confirm. It is NOT egress; it's a server-served reserved file, same
trust as `_rela.js`.

## The real cost (flagged)

The tokens currently live **inline in App.vue**, not a standalone file. To serve
them without drift, extract the `:root` / `:root.dark` token blocks into a
shared source that BOTH App.vue and the `_rela.css` generator read (or generate
`_rela.css` from the same source). A duplicated copy WILL drift — don't. This is
a small but real refactor of the SPA's theming layer and the main effort here.
`.input`/`.card` don't exist as utilities yet, so they're freshly authored (no
extraction, but they become a new app-facing contract to keep stable).

## Scope

- IN: extract token source; generate/serve `_rela.css` (`:root` + `:root.rela-dark`
  + `.btn`/`.input`/`.card`) at the reserved per-app path; SDK toggles
`rela-dark` from host theme; docs (authoring section: opt-in link, available
tokens, the 3 classes, theme behavior); tests (served with text/css under the
app CSP; dark class flips; an example app using it).
- OUT: any component-shaped class beyond the atomic three; a full design-system
package; letting apps load arbitrary external stylesheets (egress stays off).
- Stays fully **opt-in** — apps that want total control just don't link it.
