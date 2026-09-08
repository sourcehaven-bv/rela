---
id: TKT-JO125X
type: ticket
title: 'App CSP: drop unsafe-inline and split the scaffold into external files'
kind: enhancement
priority: low
status: done
---

## Problem

`appCSP()` sets `script-src` and `style-src` to include `'unsafe-inline'`, which
disables a defense-in-depth layer against XSS *inside* an app: if an app injects
untrusted data into the DOM (e.g. via `innerHTML`), a strict CSP would still
block the resulting `<script>` tags. With `unsafe-inline` that injection runs.

Reported as [#1025](https://github.com/sourcehaven-bv/rela/issues/1025) from the
IB review of #1012. Severity: Low. Grounds: POLICY-015 §3 (OWASP A03).

This is not a weakening of the main boundary — an app already runs with
`allow-scripts`. The actual data boundary is path-scoping + `connect-src 'none'`
+ the bridge, and none of that changes here.

## Why this was previously parked

`unsafe-inline` was deliberate (TKT-VEJ39W), and the blocker was that rela's own
scaffold emits inline markup: `rela apps new` writes an `index.html` with an
inline `<style>` and `<script>`. A strict CSP would break the app rela generates
for you before you write a line of your own code.

The decision: **the scaffold should work out of the box**, so fix the scaffold.

## Verified empirically before implementing

Built a server with the strict CSP and loaded a split-file app in a real browser
(headless Chrome):

- external `app.css` applied (h1 17.5px, body padding 21px)
- external `_rela.js` loaded (`window.rela` defined)
- injected inline `<script>` did NOT run; violation `script-src-elem <- inline`
- injected inline `<style>` did NOT apply; violation `style-src-elem <- inline`

Control on the CURRENT CSP: the same injected inline script DOES run, with zero
violations. So the change delivers exactly the protection the issue asks for.

## Approach

1. Scaffold writes three files: `index.html`, `app.css`, `app.js` — no inline.
2. Reference app `tickets/apps/ticket-counter/` split the same way, including
its two inline `style=` attributes (pure layout, moved to classes).
3. `appCSP()` drops `'unsafe-inline'` from `script-src` and `style-src`.
4. Docs state the rule so app authors know why, and that `style=` attributes
would need `unsafe-hashes` (i.e. use classes instead).

## Acceptance criteria

- AC1 A scaffolded app loads and runs under the strict CSP with no violations.
- AC2 The reference app renders identically to before.
- AC3 `appCSP()` contains no `'unsafe-inline'`; a test pins it.
- AC4 An inline `<script>` in an app is blocked (regression guard).
- AC5 External `.css`/`.js` still serve with correct content types.
- AC6 Docs tell app authors to use external files and classes.
