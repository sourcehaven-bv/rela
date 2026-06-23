---
id: TKT-F5FDEQ
type: ticket
title: Optional rela design tokens + base controls CSS for custom apps (_rela.css)
kind: enhancement
priority: low
effort: m
status: done
---

Follow-up to TKT-9AW5RF (custom folder apps). Let an app opt into rela's look so
it renders consistently with the SPA, without coupling to component internals.

**STATUS: done.** Shipped as `_rela.css` served at
`/api/v1/_apps/<id>/_rela.css`, opt-in via `<link rel="stylesheet"
href="_rela.css">`.

## What shipped

- **Theme tokens** (`:root` / `:root.dark` custom properties) — extracted from
App.vue into `frontend/src/styles/tokens.css` (the SPA imports it; the Go binary
embeds a byte-identical copy `apps_tokens.css`, pinned by
`TestAppTokensCSSInSyncWithFrontend` so they can't drift).
- **Three atomic controls**: `.btn`/`.btn-primary`, `.input`, `.card`
(`appBaseControlsCSS` in `apps_css.go`). Nothing component-shaped — a test
asserts no `.table`/`.modal`/`.select` creep in.
- **Dark mode follows the host automatically.** Implementation note vs. the
original plan: I used the **same `:root.dark` selector as the SPA** (not a
separate `rela-dark`), which is what lets the identical `tokens.css` work
verbatim in both contexts. The SDK toggles `dark` on the app's `<html>`, seeded
at the handshake reply and pushed live via a `rela:theme` message when the host
theme changes (verified live in the browser — switching the host theme flips the
app).
- **No CSP change**: the app's existing path-scoped `style-src` already permits
`_rela.css` (served from the app's own path). Reserved like `_rela.js`; apps
can't shadow it.
- Docs in the data-entry guide; example app (`ticket-counter`) opts in and uses
the tokens + `.btn`/`.card`. Go + e2e tests cover serving, content-type,
tokens/controls presence, and the drift guard.

Fully opt-in — an app that wants total control of its look just doesn't link it.

## Line for future additions

Publish a class only if it's pure presentation with one obvious HTML shape AND
we'd freeze its appearance as a public contract. Tokens are safe forever
(renaming `--text-color` would break the SPA anyway); components past the atomic
three are not — apps build those from the tokens.
