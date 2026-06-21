---
id: PLAN-0SXMBD
type: planning-checklist
title: 'Planning: Custom apps: sandboxed-HTML extensions served in the data-entry SPA via a REST-API bridge'
status: done
---
<!-- @managed: claude-workflow v1 -->

> **DESIGN-REVIEW REVISION (post /design-review).** Net changes: (1) **No
> `internal/acl` changes** — an app is a UI shell that acts as the logged-in
> user; every call goes through the existing read gate / entitymanager write
> authz, so it can only do what the user already can. No `OpRunApp`/`AppSubject`.
> (RR-RBAZSX) (2) **CSP must be injected as a `<meta http-equiv>` into the
> served HTML** — the HTTP response header is stripped when HTML is loaded via
> `iframe.srcdoc`. (RR-ZOLWMD, critical) (3) **Bridge surface corrected** to the
> real endpoint set incl. relations + schema/config; `trace` dropped (no REST
> endpoint). (RR-YLG57K) (4) `_apps/{id}` stays under `/api/`; origin-`null`
> blocking documented as the deliberate backstop; `allow-same-origin` never set.
> (RR-68HMJP)

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** full vertical slice, host-credentialed identity (iframe has no
network access; the SPA host page makes all `/api/v1/*` calls under the existing
same-origin session, so an app acts as the logged-in principal and **can only do
what that user can already do**).

IN scope:

1. `apps/` filesystem loader (traversal-resistant `os.OpenRoot`, mirrors
`internal/script/action.go`'s `actionsDir`).
2. `apps:` config block in `data-entry.yaml`
(`dataentryconfig.Config.Apps map[string]AppDef`) + validation (id regex, file
existence, optional `csp_origins`). **Config presence is the only gate on app
availability.**
3. `GET /api/v1/_apps/{id}` — serves app HTML with a **`<meta http-equiv>` CSP
injected into the body** (load-bearing for srcdoc) PLUS the same CSP as an HTTP
header + `nosniff` (belt-and-braces; mirrors `handleAPIGetThemeLogo`). CSP:
`default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline';
img-src data: blob:` plus configured `csp_origins` merged into
`connect-src`/`img-src`.
4. Vue `/app/:id` route + `AppHostView.vue`: fetch `_apps/{id}` HTML, render in
`<iframe sandbox="allow-scripts allow-forms" srcdoc=...>` (**never
`allow-same-origin`**), set up a `MessageChannel`, broker a fixed RPC surface to
the existing axios `/api/v1/*` client.
5. **Bridge RPC surface (closed allow-list, corrected — RR-YLG57K):** reads =
`schema`, `config`, `list`, `get`, `search`, `analyze`, `templates`,
`documents`, `position`; entity writes = `create`, `update`, `delete`; relation
writes = `relationCreate`, `relationUpdate`, `relationDelete`
(`/{plural}/{id}/relations/{relType}[/{targetId}]`); `action` → `POST
/_action/{id}`. `rename` only if a real endpoint exists (verify; else defer).
`trace` dropped (no REST endpoint). Each method maps to one existing api-client
call — **no path passthrough.**
6. In-iframe `rela.*` SDK (static asset) wrapping the MessageChannel port.
7. Sidebar lists configured apps.
8. Docs + an example app under a fixture `apps/` dir.
9. e2e (read + write + relation-link), Go handler/loader tests, frontend bridge
unit tests.

OUT of scope: any `internal/acl` change (apps inherit user perms — RR-RBAZSX);
in-app editor; query DSL / non-REST read path; per-app identity; Lua on read
path; `transition:*` verbs; SSE forwarding to apps (fast-follow).

**Acceptance Criteria:**

1. **Injected `<meta>` CSP.** `GET /_apps/{example}` returns HTML with a
`<meta http-equiv="Content-Security-Policy">` containing `default-src 'none'` in
`<head>`, plus the matching header + `nosniff`. *Test:* handler test (meta
present+well-formed, headers).
2. **Bad/unknown id → 400/404.** *Test:* handler test.
3. **Traversal-resistant load.** Out-of-`apps/` config rejected at load. *Test:*
loader test.
4. **App acts only as the user.** Bridge call resolves identically to the host
page doing it directly (denied write → 403 to app + audit row; hidden read →
not-found). *Test:* bridge unit test + e2e under declarative ACL.
5. **Closed allow-list enforced.** Unknown/path-like method → structured error,
no fetch. *Test:* bridge unit test.
6. **Iframe isolation + bridge-only path.** origin-`null` (no
`allow-same-origin`); direct `/api/` fetch blocked (CSP + server origin-null);
host ignores `window` `postMessage`. *Test:* CSP/header assertions + bridge unit
test + e2e.
7. **Relations linkable.** `relationCreate`/`relationDelete` link two entities.
*Test:* e2e.
8. **Same-origin enforced.** Cross-origin bridge POST → 403; host page passes.
*Test:* extend middleware test.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-HZ9MMR (done)

**Existing Solutions:** Reference = Datasette Apps (sandbox iframe + strict CSP

- MessageChannel), but rela's REST API replaces the SQL bridge. Divergence found
in review: Datasette serves apps as top-level docs (HTTP CSP header applies); we
load via srcdoc so CSP must be `<meta>`. Codebase prior art (confirmed):
`handleAPIGetThemeLogo` (handlers_theme.go:39-66, header-based,
direct-nav-only); `actionsDir` + `os.OpenRoot` (script/action.go:20,39);
`handleV1Action` (actions.go:54); read gate (readgate.go,
acl.Request.PermitsRead); same-origin + origin-null
(middleware_security.go:141-234); router (frontend/src/router/index.ts); axios
`baseURL:'/api/v1'`. No existing postMessage/MessageChannel/srcdoc/http-equiv
usage — bridge + meta-CSP are net-new.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

```
apps/<id>.html (disk)
  → GET /api/v1/_apps/{id}  [config lookup + inject <meta> CSP]   (Go)
  → AppHostView.vue: <iframe sandbox="allow-scripts allow-forms" srcdoc=...>
                     (host, same-origin; NEVER allow-same-origin)
  → MessageChannel(port1=host, port2=iframe)
  → iframe rela.*  --port-->  host RPC dispatcher (closed allow-list)
  → host axios → /api/v1/*  [readGate / entitymanager / _action]
```

*Authorization:* none app-specific. The iframe can't reach the network (CSP
`default-src 'none'`; origin-null 403s at the server anyway — a deliberate
backstop). Every API call is made by the host page under the user's session and
gated like the SPA's own (reads via readGate, writes via entitymanager re-auth +
audit). App = user. App availability gated by config presence only. Caveat: any
authenticated user can load any declared app's inert HTML/JS (project config via
git/PR, not user data).

*CSP delivery (critical fix):* the handler injects `<meta
http-equiv="Content-Security-Policy">` into `<head>` — the control that applies
inside srcdoc (the HTTP header is stripped by the host fetch). Header still set
(belt-and-braces). `csp_origins` merged into both. Sandbox without
`allow-same-origin` keeps the app origin-null.

*Bridge* maps each allow-listed method to one existing api-client call — never
an arbitrary path.

**Files (add/modify):** Backend: `dataentryconfig/config.go` (`Apps`/`AppDef`),
`dataentryconfig/validate.go` (id regex, file, csp_origins), `dataentry/apps.go`
(new — OpenRoot loader + `<meta>` injector), `dataentry/apps_handler.go` (new —
`handleV1App`), `dataentry/api_v1.go` (register `_apps/`), app state (surface
apps for sidebar/list). NO `internal/acl` change, NO
`affordances.go`/`translateVerb` change, NO `middleware_security.go` change
(just a clarifying comment). Docs: `docs/data-entry.md`,
`docs/data-entry/api-reference.md`. Frontend: `router/index.ts` (`/app/:id`),
`views/AppHostView.vue` (new), `api/apps.ts` (new), `bridge/relaBridge.ts` (new
host dispatcher), `bridge/app-sdk.js` (new in-iframe SDK), sidebar, bridge unit
tests.

**Alternatives rejected:** query DSL (A2), Lua read funcs (A3), apps-as-entities
(C2), per-app identity, and the `OpRunApp` write-Op model (breaks read-only +
panics on sealed Subject; unnecessary since apps inherit user perms).

**Dependencies:** no new Go modules; frontend none beyond axios + vue-router;
MessageChannel is standard.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** app id (path) — regex `^[a-z0-9_-]{1,64}$`, else
400; app file (config, not request) — `os.OpenRoot`-scoped, `..`/abs/symlink
rejected at load; `csp_origins` (config) — validated as origins, merged only
into `connect-src`/`img-src`; bridge RPC — method must be in the fixed
allow-list, payload validated per-method, unknown → structured error (no
passthrough), host listens only on its owned `port1` and ignores `window`
`postMessage`.

**Security-Sensitive Operations:** serving user HTML (`<meta>` CSP in body +
header + `nosniff` + sandbox without `allow-same-origin` → origin-null, no
cookies/storage/parent-DOM/egress); backstop = origin-null rejected by
`requireSameOrigin` (middleware_security.go:216-218), so the iframe can't reach
`/api/` directly — host bridge is the sole path; file access OpenRoot-scoped to
`apps/`; authz = readGate/entitymanager (app = user); errors → unknown app 404,
bridge generic codes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1/2 handler test (meta+headers, 400/404); AC3 loader
traversal test; AC4 bridge unit + e2e under declarative ACL; AC5 bridge unit
(unknown method → error, no fetch); AC6 CSP/header + non-paired-message + e2e;
AC7 e2e relation link; AC8 extended middleware test.

**Edge Cases:** empty `apps:` → 404 + empty sidebar; file-on-disk-not-declared →
404; declared-but-missing-file → startup validate error; no-`<head>` HTML →
injector synthesizes head (explicit test); app's own `<meta>` CSP → server
injects its own, browser intersects (document); oversize file → max-bytes cap;
unicode/null id → regex rejects; concurrent loads → read-only snapshot; postgres
build → `apps/` on filesystem identically.

**Negative Tests:** bad id (400), unknown id (404), traversal config (load
error), unknown bridge method (structured error), cross-origin write (403),
iframe direct fetch blocked (CSP + origin-null), no-head HTML (meta still
injected).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** CSP-in-srcdoc (was critical, resolved — inject `<meta>`, handler test
pins it); bridge → generic proxy drift (closed allow-list + unit test,
documented no-passthrough rule); `<meta>` injection into malformed/headless HTML
(robust injector + edge tests); relations ergonomics (model on existing
sub-resource endpoints, e2e covers a real link).

**Effort:** L (backend S now that ACL work is dropped; frontend host+bridge M;
docs+e2e S).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md — "Custom apps" section (authoring, `apps:` config,
`rela.*` bridge API + full method list, security model incl. meta-CSP + sandbox
- origin-null backstop).
- [x] docs/data-entry/api-reference.md — `GET /_apps/{id}` (no new ACL verb).
- [x] CLAUDE.md (project + internal/dataentry) — apps mechanism, closed-method
bridge rule, `apps/` asset dir alongside `actions/`/`scripts/`, the
meta-CSP-for-srcdoc gotcha.
- [x] ~~README.md~~ (N/A: feature documented under data-entry docs, not a
project-level change).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-ZOLWMD (critical, CSP/srcdoc → addressed:
`<meta>` CSP); RR-RBAZSX (significant, OpRunApp write-Op → addressed: drop ACL
change, app=user); RR-YLG57K (significant, bridge surface → addressed: corrected
closed allow-list incl. relations + schema/config, trace dropped); RR-68HMJP
(minor, `_apps/{id}` placement → addressed: srcdoc-only, origin-null backstop
documented). All four `addressed`.
