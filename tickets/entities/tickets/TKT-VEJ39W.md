---
id: TKT-VEJ39W
type: ticket
title: 'Multi-file zip apps: serve apps/<id>.zip as a directory with path-scoped CSP (bridge preserved)'
kind: enhancement
priority: medium
effort: l
status: backlog
---

Follow-up to TKT-9AW5RF. Let a custom app be a **zip bundle** (`apps/<id>.zip`
with at least `index.html`) so apps can ship multiple files (`app.js`,
`style.css`, images, fonts).

## DECISION (supersedes TKT-9AW5RF's srcdoc model): one unified serving model

Single-file and zip apps are served the **same way** — a single HTML file is
just the one-entry case. There is no `srcdoc` path. Both are served from a real
URL with a CSP **header**:

- **iframe `src="/api/v1/_apps/<id>/"`** (not `srcdoc`). Real origin → an app's
sub-resources (`<script src=app.js>`, css, images, fonts) load normally; a
single-file app just has no sub-resources.
- **CSP via HTTP response HEADER, not `<meta>`.** Since the document is served
over a real HTTP request, the header applies (and `frame-ancestors`/`sandbox`
CSP directives are header-only anyway). No `<meta>` injection, no tokenizer
CSP-injection — that whole srcdoc workaround (TKT-9AW5RF / RR-ZOLWMD) goes away.
- **Storage:** `apps/<id>.html` (single doc) OR `apps/<id>.zip` (bundle). Zip is
read on demand with `archive/zip`, never extracted to disk.
- **SDK delivery:** still inject the SDK `<script>` into `index.html` (or the
single `.html`) server-side via the `x/net/html` tokenizer, so the app's JS has
`window.rela`. (Only the SDK is injected now; CSP is the header.)

### Security consequence of unifying (accepted)

The shipped single-file model rendered apps at **origin-`null`** (srcdoc +
sandbox without allow-same-origin), so the same-origin middleware rejected any
`/api/` call regardless of CSP — **two independent guards**. Unifying makes
every app **same-origin**, so the **path-scoped CSP is the only thing** keeping
an app off `/api/`. One guard. A CSP authoring/serving bug = full bridge bypass.
Mitigations to design:
- **Server-side second guard via `Sec-Fetch` headers:** an app-iframe request
(identifiable by `Sec-Fetch-Dest`/`Sec-Fetch-Site` + a `/_apps/` referer; not
JS-spoofable) may be blocked from reaching non-`/_apps/` `/api/` paths at the
middleware, restoring defense-in-depth without origin-`null`. Evaluate.
- **Separate app origin** remains the strong-isolation escape hatch for
deployments that want it (out of scope here; infra).

## The path-scoped CSP header (the whole boundary now)

Set on EVERY served response (index + sub-resources):
```
Content-Security-Policy:
  default-src 'none';
  script-src  /api/v1/_apps/<id>/ 'unsafe-inline';
  style-src   /api/v1/_apps/<id>/ 'unsafe-inline';
  img-src     /api/v1/_apps/<id>/ data: blob:;
  font-src    /api/v1/_apps/<id>/;
  connect-src 'none';      ← blocks app fetch/XHR/WS → bridge is the only data path
  form-action 'none';
  base-uri    'none';
  frame-src 'none'; child-src 'none';
  frame-ancestors 'self';
```

Path-scoping every resource directive to the app's own subdir is
**load-bearing**: same-origin means a bare `'self'` would let `<img
src="/api/v1/tickets/secret">` pull API data; scoping to `/_apps/<id>/` stops
resources coming from `/api/`. `connect-src 'none'` blocks fetch/XHR/WS;
`form-action 'none'` + sandbox (no `allow-top-navigation`) block form/nav exfil.
The `MessageChannel` bridge is unaffected by `connect-src` (not a network
connection), so it stays the sole API route — preserving the per-app capability
chokepoint (future read-only/restricted-perms apps enforce at the bridge).

## Why keep the bridge (unchanged from TKT-9AW5RF)

rela has no browser-side session/cookie (identity is server-stamped from
`$RELA_DATAENTRY_USER` / trusted `--principal-header`), so the bridge isn't
guarding a stealable credential. Its value is the per-app capability chokepoint:
the place to later enforce "this app may do less than the user can." Same-origin
direct-`fetch` can't be restricted below the user's ACL without per-app
principals. The bridge keeps that seam.

## Security surface (the real work)

- **Zip-slip / traversal:** reject any entry whose cleaned path escapes root
(`../`), even though we never write to disk — a request could name it.
- **Content-Type** inferred from extension must be correct (else `script-src`
won't run JS / `nosniff` blocks it).
- **Limits:** per-entry size cap, total uncompressed cap (zip-bomb), entry count.
- **CSP is the entire boundary:** header on EVERY response, exact path-scoping.
Exhaustive tests on every directive + exfil channel (img/form/nav/connect) + a
test that the header (not a meta) carries it + (if built) the Sec-Fetch guard.
- **Handshake:** host→iframe `postMessage` handshake works same-origin; the
SDK's `ev.source === window.parent` check still holds.

## Scope

- IN: unify serving on `src=`+header (revises TKT-9AW5RF's srcdoc/meta — see
that ticket); `apps/<id>.zip` discovery + `archive/zip` per-entry handler;
single-file served via the same path; index SDK-injection; path-scoped CSP
header on all responses; zip-slip + limits; evaluate the Sec-Fetch second guard;
tests (unit + e2e + CSP exfil-channel).
- OUT (later/deliberate): separate app origin; per-app permissions (the bridge
seam this preserves; the restriction itself is its own feature); heavy
build-tool `dist/` bundles beyond what path-scoped CSP allows.
