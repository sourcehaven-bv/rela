---
id: TKT-ZDRS
type: ticket
title: Bundle Font Awesome locally so EasyMDE doesn't fetch it from a CDN
kind: enhancement
priority: medium
effort: s
status: done
---

EasyMDE auto-injects a `<link rel="stylesheet"
href="https://maxcdn.bootstrapcdn.com/font-awesome/latest/css/font-awesome.min.css">`
at runtime unless `autoDownloadFontAwesome: false` is passed. The data-entry SPA
depends on the resulting `fa fa-*` glyphs for every default EasyMDE toolbar
button (bold, italic, heading, link, code, quote, preview, side-by-side,
fullscreen, guide — see
`frontend/src/components/forms/MarkdownEditor.vue:71-90`).

Consequences:

- The Go binary is no longer self-contained: opening the markdown editor on an air-gapped machine, behind a proxy, or with maxcdn.bootstrapcdn.com unreachable renders unlabeled blank toolbar buttons (only the `title=` attribute identifies them).
- Every editor mount makes an uncached cross-origin request to a CDN we don't control; future maxcdn outages, deprecations, or TLS-cert changes silently degrade the UI.
- This contradicts the deployment story documented for `data-entry-ui` and `FEAT-021` (CLI binary dependency optimization) — assets are otherwise embedded via `//go:embed` in `internal/dataentry/static.go`.
- It is also the root concern flagged but deferred in `RR-UMGR` on TKT-I5NO.

Note: spell-checker is currently `false` so the
`cdn.jsdelivr.net/codemirror.spell-checker/latest/en_US.{aff,dic}` URLs EasyMDE
would otherwise fetch are not requested today; this ticket scope is Font Awesome
only. If spell-check is ever re-enabled, the dictionaries must be bundled too.

Acceptance criteria:

1. After build, opening any data-entry form with a markdown editor makes **zero** network requests to `maxcdn.bootstrapcdn.com` (verified in browser DevTools Network panel with cache disabled, and via an e2e assertion).
2. The EasyMDE toolbar buttons still render their icons (bold, italic, heading-1/2/3, unordered-list, ordered-list, link, code, quote, preview, side-by-side, fullscreen, guide) identically to today.
3. Font Awesome assets are served from the same origin as the SPA (i.e. embedded in the Go binary via `//go:embed`).
4. Running `rela-server` on a machine with the loopback interface as the only network route still renders the toolbar icons correctly.
