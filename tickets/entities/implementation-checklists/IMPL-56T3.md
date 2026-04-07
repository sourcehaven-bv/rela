---
id: IMPL-56T3
type: implementation-checklist
title: 'Implementation: Drop user-visible /v2/ URL prefix and remove stale HTMX app.js'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: pure deletion + config change refactor; no new logic to test)
- [x] ~~Integration tests written~~ (N/A: existing dataentry test suite exercises the router; updated `middleware_security_test.go` to drop the obsolete `/v2/` exempt-path assertion)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no new tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A)
- [x] ~~Only specifying values that matter for the test~~ (N/A)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

**Built server** (`just build-server`) produces `bin/rela-server` (27MB). Ran it
against `prototypes/data-entry/project` on port 8765 and verified with curl:

| Request | Result | AC |
|---------|--------|----|
| `GET /` | 200, SPA shell with `href="/favicon.svg"` and `src="/assets/index-BJ9jBuso.js"` — no `/v2/` in the HTML | AC 1 |
| `GET /favicon.svg` | 200, 12614 bytes | AC 2 |
| `GET /list/ticket` (deep link) | 200, SPA shell served via catch-all | AC 3 |
| `GET /assets/index-BJ9jBuso.js` | 200, 171140 bytes (SPA bundle) | AC 1 |
| `GET /v2/` | 200, still serves SPA shell via catch-all (Vue router will treat it as an unknown route) — no hard 404 for stale bookmarks, which is a bonus | — |
| `GET /api/v1/_schema` (no Origin) | 403 — security middleware still correctly blocks cross-origin API calls (unchanged behaviour) | AC 2 security |

**Built output inspection:** `cat internal/dataentry/static/v2/index.html` after
`just build-frontend` shows `/favicon.svg` and `/assets/...` paths — no `/v2/`
anywhere. Vite log showed `base: '/'` taking effect (no more "Use /v2/ base in
production build" comment).

**Embedded `app.js` verification:**

- `strings bin/rela-server | grep -c "Data Entry App JavaScript"` → **0** (unique marker from the deleted `app.js` header)
- `strings bin/rela-server | grep -c "EasyMDE"` → 3, but these come from the Vue `MarkdownEditor.vue` bundle, not from the deleted `app.js` (the Vue component also uses EasyMDE; the planned acceptance criterion wording was imprecise — the correct check is the unique marker)

**Binary size change:** `bin/rela-server` is 27MB. A clean baseline build is not
available to diff against without re-running on develop, but the `static/app.js`
removal saves ~74KB of uncompressed, embedded-via-`//go:embed all:static/*`
content. No Go code change was needed because the `embed` uses a glob.

**Vue unit tests:** 286/286 passed (`cd frontend && npm run test:run`).

**Go tests:** all packages pass via `go test -race ./...` including the updated
`internal/dataentry` (`middleware_security_test.go` no longer asserts `/v2/` is
an exempt path).

**Lint:** `just lint` clean.

**rela analyses:** all three analyses (`cardinality`, `orphans`, `properties`)
pass. Note: I updated (rather than deleted) the `codemirror-textarea-sync`
automated-measure to point at `frontend/src/components/forms/MarkdownEditor.vue`
with a description reflecting the Vue v-model sync pattern. The
deletion-then-refresh surfaced that `BUG-005` has a required `adds-measure`
relation to it, so leaving the measure in place but updating its `location` +
`description` keeps the traceability intact and honest (the fix still exists,
just in a different component).

## Pre-existing issues found during implementation (NOT caused by this ticket)

These are out of scope but worth filing as follow-ups:

1. **e2e fixture broken by security hardening (PR #318).** `frontend/e2e/fixtures.ts:55` waits for the backend to respond to `GET /api/v1/_config`, but the new `requireSameOrigin` middleware returns 403 when Node's `fetch` sends no Origin header. `isServerRunning` treats 403 as "not yet running" and times out after 30s. Every e2e test fails with "Server at … did not start within 30000ms". Verified on clean develop (stash + test): the bug exists on `c68dc2b` without any of my changes. CI does not run e2e tests (`.github/workflows/ci.yml` has no `test:e2e` step), which is why this slipped through PR #318.
   - **Suggested fix for the follow-up bug ticket:** set `Origin: http://localhost:<port>` on the probe fetch, and/or treat 403 as "server is up" in `isServerRunning`.

2. **`just test` fails locally due to Go toolchain mismatch when using `-cover`.** Homebrew's `go1.25.6` + the project's `go.mod` toolchain directive `go1.25.8` produces `compile: version "go1.25.8" does not match go tool version "go1.25.6"` errors on `cmd/rela` *only when -cover is enabled*. Plain `go test ./...` works fine. Pre-existing environment issue, unrelated to this ticket.

## Quality

- [x] Code follows project patterns (matches existing `spaHandler` and `staticFS` naming conventions)
- [x] No security issues introduced — change only REMOVES an HTTP route, never adds one. The security middleware's `sensitivePathPrefixes` list is unchanged.
- [x] No silent failures — the `panic("embedded SPA filesystem: ...")` on `fs.Sub` failure is a startup-time panic that matches the existing `staticFS` pattern; any embedded FS resolution bug fails fast at server startup, not silently at runtime.
- [x] No debug code left behind
