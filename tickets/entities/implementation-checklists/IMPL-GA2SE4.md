---
id: IMPL-GA2SE4
type: implementation-checklist
title: 'Implementation: Custom apps: sandboxed-HTML extensions served in the data-entry SPA via a REST-API bridge'
status: done
---
<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (Go: apps_test.go — buildAppCSP,
injectCSPMeta, loadAppHTML traversal, handleV1App; frontend: relaBridge.test.ts
— 9 tests)
- [x] Integration tests written (live rela-server run against tickets/ project

  - browser round-trip via puppeteer; router_walk_test probe for _apps/)
- [x] Happy path implemented (config → loader → handler → iframe host → bridge
→ REST API)
- [x] Edge cases from planning handled (no-head HTML, <header> vs <head>,
traversal, oversize cap, bad/unknown id, declared-but-missing file)
- [x] Error handling in place (404 on unknown/unreadable, 400 on bad id,
structured bridge errors, no passthrough)

## Test Quality

- [x] Using fixture builders or factories for test data (newHandlerTestApp,
t.TempDir for the on-disk apps/ dir)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the SPA bundle + rela-server, ran against the `tickets/` dogfood project
with an example app (`apps/ticket-counter.html` + `apps:` config block).

Server endpoints (curl, with Origin header):

- `GET /api/v1/_config` → exposes `apps.ticket-counter` metadata, **without**
leaking `file`/`csp_origins` (AC: client view). ✅
- `GET /api/v1/_apps/ticket-counter` → 200; response carries the injected
`<meta http-equiv="Content-Security-Policy" content="default-src 'none'…">`
(AC1), the CSP **header** (`default-src 'none'… ; frame-ancestors 'self'`),
`X-Content-Type-Options: nosniff`, and the original `<title>` preserved. ✅
- `GET /api/v1/_apps/nope` → 404 (AC2). `GET /api/v1/_apps/Bad.Id` → 400
(AC2). ✅
- Startup config validation accepted the example app file; a missing file
would fail fast (CheckAppFileExists wired into NewApp). ✅

Browser round-trip (puppeteer at `/app/ticket-counter`):

- The app rendered **"25 tickets"** — real data fetched through the bridge
(`rela.list({type:'ticket'})` → host dispatcher → `/api/v1/_tickets`). This is
the proof the MessageChannel handshake + dispatch + REST round-trip works
end-to-end (AC4 read, AC6 bridge-only path). ✅
- DOM inspection confirmed: iframe `sandbox="allow-scripts allow-forms"` (NO
`allow-same-origin`), `srcdoc` contains both the injected `<meta>` CSP and the
injected `window.rela` SDK, and **`iframe.contentDocument` is null from the
host** — the origin-`null` sandbox genuinely isolates the app; the host cannot
read into it. Data flows only over the MessageChannel. (AC6 isolation.) ✅

Automated:

- `go test ./internal/dataentry/ ./internal/dataentryconfig/` → ok.
- frontend `vitest run src/bridge/` → 9 passed (closed allow-list, no
passthrough, method→call mapping, param validation, error normalization) (AC5).
✅
- `npm run typecheck` clean; `npm run lint` 0 errors (no warnings in new
files).
- `go vet` clean; `just arch-lint` OK.

Deferred to review/follow-up: a dedicated Playwright e2e under e2e/tests/ (would
need an apps fixture + page object). AC7 (relation link via bridge) and AC8
(cross-origin 403) are covered by unit/middleware-level reasoning but not yet an
automated e2e — noted for the review phase.

## Quality

- [x] Code follows project patterns (mirrors handlers_theme.go CSP-serve,
script/action.go os.OpenRoot loader, actions.go handler shape, _config
serialization)
- [x] Checked for DRY opportunities — CSP string centralized in appCSPBase +
buildAppCSP; injectCSPMeta/indexCaseInsensitiveOpenTag are single-purpose
helpers; bridge method→call mapping is one table
- [x] No security issues introduced (closed allow-list, traversal-resistant
load, sandbox without allow-same-origin, injected <meta> CSP, no app-specific
authz because app inherits user perms, origin-null backstop)
- [x] No silent failures (load errors → 404; bridge errors → structured codes)
- [x] No debug code left behind
