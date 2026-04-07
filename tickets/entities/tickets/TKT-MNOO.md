---
id: TKT-MNOO
type: ticket
title: Drop user-visible /v2/ URL prefix and remove stale HTMX app.js
kind: refactor
priority: medium
effort: s
status: done
---

The Vue SPA is built with Vite `base: '/v2/'`, so production URLs are
`/v2/list/...` etc. This was a backward-compat shim during the HTMX→Vue
migration and no longer serves a purpose — there are no v1 routes left to share
the namespace with.

Additionally, `internal/dataentry/static/app.js` (74KB) is the original HTMX-era
app, still embedded in the binary but referenced from nowhere.

## Scope (user-visible only)

- `frontend/vite.config.ts`: `base: '/'` for production builds; consider renaming `outDir` from `static/v2` to a neutral name (e.g. `static/spa`)
- `frontend/index.html`: change favicon href from `/v2/favicon.svg` to `/favicon.svg`
- `internal/dataentry/router.go`: remove `mux.Handle("/v2/", ...)` legacy alias and the v1 backward-compat comment; update `fs.Sub` path
- Delete `internal/dataentry/static/app.js` (74KB HTMX leftover, embedded but unreferenced)
- `.gitignore`: update `internal/dataentry/static/v2/` entry to match new directory name
- `frontend/CLAUDE.md`: update documented build path

## Out of scope

- `/api/v1/` REST API path stays — server-only, no harm in keeping it
- Internal Go naming (`handleV1*`, `api_v1.go`, `registerAPIV1Routes`) stays
- `/api/v1/_events` SSE alias stays
