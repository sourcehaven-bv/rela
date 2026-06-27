---
id: RES-HZ9MMR
type: research
title: Custom "apps" extension for the data-entry web app (Datasette-Apps approach)
summary: 'DECIDED: filesystem-backed (apps/<id>.html) sandboxed-iframe apps; MessageChannel bridge exposes ONLY the existing ACL-gated REST API (reads + entitymanager writes + _action); future Lua reads exposed via the same REST API. New OpRunApp ACL gate. No query DSL, no read-path Lua.'
status: done
---
## Problem

We want to let users **extend the data-entry web app with custom "apps"** —
self-contained HTML+JS applications, authored per-project, that render a bespoke
UI over the entity graph (dashboards, specialized data-entry forms, domain
mini-tools) without forking the Vue SPA or shipping Go code.

The inspiration is Simon Willison's **Datasette Apps**
(https://simonwillison.net/2026/Jun/18/datasette-apps/, HN item 48593731).
Datasette serves user-authored single-file HTML apps inside a locked-down iframe
and gives them a narrow JS bridge to the database: `datasette.query(db, sql,
params)` for reads and `datasette.storedQuery(db, name, params)` for writes
(writes go only through pre-registered named queries). Isolation is `<iframe
sandbox="allow-scripts allow-forms" srcdoc=...>` + a strict CSP (`default-src
'none'`) + `MessageChannel` transport (not raw `postMessage`) + an origin
allow-list gated behind a staff-only permission.

The question this research answers: **what is the right shape for an equivalent
feature in rela**, given that rela differs from Datasette in three load-bearing
ways — (1) there is no SQL surface; reads are typed graph queries via a JSON
API; (2) the write path is already a re-authorizing, audited `entitymanager`
with `_actions` affordances; (3) storage is markdown-on-disk with a postgres
backend variant, and several non-entity asset directories (`actions/`,
`scripts/`, `templates/`, `acl.yaml`) already live on the filesystem in all
build tags.

Four sub-decisions: **(A) read-bridge shape**, **(B) write path**, **(C) app
storage**, **(D) security recipe**.

## Context

Findings below are confirmed against source. The headline result: rela **already
has** most of the primitives Datasette had to build, so this is largely a
transplant-and-wire exercise, not greenfield.

### Existing primitives we can reuse

- **Traversal-resistant per-project asset loader.** `actions/` (Lua actions,
`internal/script/action.go:20`), `scripts/` (MCP Lua,
`internal/mcp/tools_lua.go:21`) and `acl.yaml`
(`internal/appbuild/appbuild.go:387`) are top-level project dirs loaded via
`os.OpenRoot(projectRoot)` — rejects `..`, absolute paths, symlinks. An `apps/`
directory follows this exact pattern. These dirs are **not** in the store and
**not** in `project.Context`; they live on the filesystem in **all** build tags
including postgres (pgstore backs only entities/relations/attachments/search —
confirmed by grep). A postgres deployment already requires a `--project` dir.
- **Per-handler CSP-sandbox for user content already exists.**
`handleAPIGetThemeLogo` (`internal/dataentry/handlers_theme.go:39`) serves
user-uploaded bytes (incl. SVG) with `Content-Security-Policy: sandbox;
frame-ancestors 'none'`, `X-Frame-Options: DENY`, `X-Content-Type-Options:
nosniff`. This is the template for serving app HTML.
- **Registered named-action mechanism already exists.**
`data-entry.yaml` has an `actions:` map (`dataentryconfig.Action{Script, Set,
Params, Label, Confirm, …}`, `internal/dataentryconfig/config.go:79`). `POST
/api/v1/_action/{id}` (`internal/dataentry/actions.go:54`, id regex
`^[a-z0-9_-]{1,64}$`) runs a Lua script (loaded from `actions/` via
`os.OpenRoot`) under `a.luaWriteDeps()` → `EntityManager` (ACL-audited), 5s
timeout, returns `{Redirect, Message, MessageType}` with an open-redirect guard
(`validateRedirect`, action.go:270). This is the direct analog of Datasette's
`storedQuery`.
- **ACL-gated read path already exists.** `internal/dataentry/readgate.go`:
`readGate{PermitsRead, PermitsReadMany, ReadQuery, SearchScope}`. Wired
per-request by `attachACLRequest` middleware (`router.go:149`) only for
`*acl.Declarative` on `/api/` paths. Single-entity GET returns **404** (not 403)
for hidden entities (existence-leak guard, api_v1.go:783). Free-text search is
scoped via `search.VisibleSearcher` (`app.go:118`). So **any read endpoint an
app calls through `/api/v1/*` is already principal-scoped.** The "no Lua on read
path" rule means only that *user Lua* never runs on reads — declarative ACL very
much does.
- **Write path re-authorizes regardless of UI hints.**
`entitymanager.Manager.authorizeAndAudit`
(`internal/entitymanager/manager.go:213`) calls `ACL.AuthorizeWrite`, emits a
`denied-write` audit row on deny. The `_actions map[string]bool`
(`affordances.go:80`, `computeActions`) is a UI hint only; `translateVerb`
(affordances.go:34) is the single `acl.WriteRequest` construction site, enforced
by a grep test (`lint_test.go`). Op constants:
`OpCreate/OpUpdate/OpDelete/OpRename` (`internal/acl/acl.go:84`).
- **Frontend.** Vite + Vue 3 + TS, axios client `baseURL:'/api/v1'`
(`frontend/src/api/client.ts:8`), `vue-router` config-driven routes
(`frontend/src/router/index.ts:11`) — a new `/app/:id` route slots in cleanly.
SSE via `useEvents.ts`. **No iframe/embed component exists today** — that host
component is net-new.
- **Security middleware already contemplates sandboxed iframes.**
`requireSameOrigin` (`internal/dataentry/middleware_security.go:145`) already
handles the literal `"null"` Origin sent by sandboxed iframes (security.go:213).
`isSensitivePath`/`sensitivePathPrefixes` (security.go:190) classify which
routes need same-origin — a new app-host route must be classified here.

### Constraints (from CLAUDE.md / arch-lint)

- Consumer-side interfaces at the call site; no service locators; capability
bundles split read/write (`ReadDeps`/`WriteDeps`).
- No new write path may touch `store.Store` directly — go through
`entitymanager.Manager`.
- Adding an ACL Op is a coordinated 5-point change: const in `acl/acl.go`,
`translateVerb` case + `perItemVerbs`/`perCollectionVerbs`, authz eval in
`acl/authz_write.go`, policy grant in `policy.go`, docs in
`docs/data-entry/api-reference.md`.
- `just arch-lint` enforces package boundaries; must pass.

## Options

The four sub-decisions are largely orthogonal; each lists options with a
recommendation.

### (A) Read-bridge shape — how the app reads graph data

**A1. Reuse the existing `/api/v1/*` JSON verbs via the bridge (RECOMMENDED).**
The bridge forwards a small set of read intents (list-by-type, get-entity,
search, trace, analyze) to the existing HTTP endpoints, which are *already*
ACL-gated by the per-request `readGate`. The app never sees SQL — it sees the
typed JSON the SPA already consumes.

- Pros: zero new authorization surface (reuses `readGate`, 404-on-hidden,
`VisibleSearcher`); reuses the exact JSON contracts the SPA already
documents/tests; smallest backend; consistent with "no Lua on read path."
- Cons: read expressiveness limited to what the JSON API exposes (no
arbitrary joins/aggregation client-side beyond what `_analyze`/list/search
give). Mitigated because the graph API is already fairly rich.
- Effort: **S** (bridge is a thin dispatcher to handlers we have).

**A2. A constrained query DSL** (a typed graph-query expression the bridge
evaluates against a snapshot, ACL-scoped).

- Pros: more expressive than fixed verbs; still no SQL/no Lua.
- Cons: net-new query language to design, parse, ACL-scope row-by-row, test,
and version — large surface, easy to get authorization wrong; duplicates much of
what list/search/trace already do.
- Effort: **L**.

**A3. Server-side Lua read functions** (app invokes a registered Lua reader).

- Pros: maximally flexible.
- Cons: **directly violates the "no user Lua on the read path" rule** (top
CLAUDE.md + `entitymanager/CLAUDE.md`) — ACL can't interpose per-row inside Lua;
per-entity cost blowup with no quota (same rationale that keeps AI off the
validation path). Rejected on principle.
- Effort: N/A (disallowed).

### (B) Write path — how the app mutates data

**B1. Bridge calls the existing CRUD endpoints; ACL re-authorizes (RECOMMENDED
baseline).** `datasette.storedQuery`-equivalent becomes
`rela.create/update/delete/rename`, dispatched to the same handlers that back
the SPA. `entitymanager` re-authorizes and audits every call; the app's
effective permissions are exactly the invoking principal's. The bridge can
surface `_actions` so the app hides controls it can't use (UI hint only).

- Pros: no new write surface; full audit + ACL for free; matches the
affordances contract.
- Cons: only the four CRUD verbs (no transition/relation-typed verbs yet —
those are already deferred project-wide).
- Effort: **S**.

**B2. B1 + registered Lua actions for richer/atomic writes (RECOMMENDED
extension).** For multi-step or domain-specific writes, the app invokes a
project-registered action via the *existing* `POST /_action/{id}` path (Lua
under `actions/`, `WriteDeps`, ACL-audited, 5s timeout). This is the exact
analog of Datasette stored queries and needs almost no new code — only a
decision on whether app-invoked actions need their own gate (see D/the gap
below).

- Pros: reuses a shipped mechanism; keeps complex write logic server-side and
reviewable; Lua stays write-only (rule-compliant).
- Cons: `_action` invocation is currently gated only by same-origin +
downstream entitymanager ACL — there is **no per-action ACL Op**. If apps can
trigger arbitrary actions, we likely want an `OpRunApp`/per-action gate (the §D
new-Op checklist).
- Effort: **S–M** (M if we add a per-action/app ACL Op).

### (C) App storage

**C1. Filesystem `apps/` directory, one file (or dir) per app, loaded via
`os.OpenRoot` (RECOMMENDED).** Mirrors `actions/`/`scripts/`/`templates/`. App =
`apps/<id>.html` (or `apps/<id>/` for multi-file). Metadata (title, label,
enabled, CSP origin allow-list) declared in `data-entry.yaml` under an `apps:`
map, mirroring `actions:`.

- Pros: free git versioning (Simon's own stated future direction for
Datasette); backend-agnostic — works identically under fsstore and pgstore
(these dirs are never in the store); reuses the traversal-resistant loader and
the per-handler CSP-sandbox response; reviewable in PRs like code.
- Cons: authoring is file-editing, not an in-app form (Datasette has a web
editor). Acceptable for a developer-facing tool; an in-app editor can come later
and just writes the same files.
- Effort: **S**.

**C2. Store apps as entities/rows.** An `app` entity type or a DB table.

- Pros: in-app CRUD authoring; goes through existing entity tooling.
- Cons: app *content* is HTML/JS, not graph data — shoehorning it into the
entity model is awkward; diverges fs vs pg; loses trivial git versioning; larger
change. Rejected.
- Effort: **M–L**.

### (D) Security recipe (transplant from Datasette)

Recommended recipe, each element mapped to an existing rela hook:

1. **Serve app HTML from a dedicated route** (e.g. `GET /api/v1/_apps/{id}`)
with the hardened headers already used for the theme logo
(`handlers_theme.go:54`): `Content-Security-Policy` with `default-src 'none';
script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:`
(Datasette's exact policy), `X-Frame-Options` handling for the host page,
`X-Content-Type-Options: nosniff`.
2. **Render in `<iframe sandbox="allow-scripts allow-forms" srcdoc=...>`** in
a net-new Vue host component on a new `/app/:id` route. No `allow-same-origin` ⇒
the app is origin-`null`, no cookies/localStorage, no parent DOM.
3. **`MessageChannel` transport** between host and iframe (not raw
`postMessage`) — channel auto-closes on navigation, matching Datasette. The host
is the only thing that talks to `/api/v1/*`; the iframe can't (CSP `default-src
'none'` blocks its own fetches), so all data flows host→bridge→API with the
host's same-origin credentials and the principal's ACL.
4. **CSP origin allow-list gated behind a new ACL Op.** Default: app can make
no outbound requests. Allowing extra `connect-src`/`img-src` origins is a
privileged operation — add an `acl.Op` (e.g. `OpSetAppCSP`) following the
5-point checklist, or an admin-configured allow-list in `data-entry.yaml` that
authors select from (mirrors Datasette's `allowed_csp_origins`).
5. **Same-origin classification.** Add the app-host and bridge routes to
`sensitivePathPrefixes` (security.go:190) so writes still require same-origin;
the existing `"null"`-origin handling (security.go:213) already covers the
sandboxed-iframe case.
6. **Reads ACL-gated, writes ACL-audited** automatically because the bridge
uses `/api/v1/*` (readGate) and entitymanager (write authz) — no new authz logic
on the data path itself.

Known gap to decide during planning: **`_action` / app invocation has no
per-action ACL Op today** (only same-origin + downstream write ACL). If apps can
trigger registered actions, decide whether to add a per-action/per-app Op
(`OpRunApp`) or rely on the downstream entitymanager gate. Recommended: add a
coarse `OpRunApp` gate so an app's *ability to exist/run* is ACL-governed, while
individual writes remain governed by the existing CRUD Ops.

## Recommendation

**Build a filesystem-backed, sandboxed-iframe "apps" feature that talks to the
existing JSON API through a `MessageChannel` bridge — reusing rela's existing
ACL-gated read path, re-authorizing write path, and per-handler CSP-sandbox
response.**

Concretely:

- **(A) read-bridge: A1** — bridge forwards typed read intents to the existing
`/api/v1/*` endpoints; no SQL, no DSL, no read-path Lua. The `readGate` already
scopes them per principal.
- **(B) write path: B1 + B2** — CRUD via existing endpoints (entitymanager
re-authorizes/audits), plus registered Lua actions via the existing
`_action/{id}` for richer server-side writes. Add a coarse `OpRunApp` ACL gate.
- **(C) storage: C1** — `apps/<id>.html` on the filesystem, declared in
`data-entry.yaml` under `apps:`, loaded via `os.OpenRoot`. Free git versioning;
identical under fsstore/pgstore.
- **(D) security: the 6-point recipe above** — `iframe sandbox=allow-scripts
allow-forms` + `srcdoc`, strict CSP (`default-src 'none'`), `MessageChannel`
transport, default-no-egress with a CSP origin allow-list gated behind a new ACL
Op, app routes added to `sensitivePathPrefixes`.

**Why this option:** it maximizes reuse of shipped, tested primitives
(traversal-resistant loader, CSP-sandbox response, readGate/VisibleSearcher,
entitymanager authz+audit, `_action` runner) and respects every standing
architectural rule (no Lua on read path, no direct store writes, consumer-side
interfaces, backend-agnostic non-entity assets). The only genuinely net-new
pieces are the Vue iframe-host component + `MessageChannel` bridge, the `GET
/_apps/{id}` HTML route, the `apps:` config block, and the `OpRunApp` ACL Op.

**Tradeoffs accepted:**

- Read expressiveness is bounded by the JSON API (no arbitrary client-side
joins/aggregation beyond list/search/trace/analyze). Acceptable; revisit with a
DSL (A2) only if real apps hit the ceiling.
- Authoring is file-editing initially, not an in-app editor. Acceptable for a
developer tool; an in-app editor is a later additive step writing the same
`apps/` files.
- Writes limited to the four CRUD verbs + registered actions (transition /
relation-typed verbs remain deferred project-wide).

**Suggested implementation slices** (for the follow-up ticket(s)):

1. `apps/` loader + `apps:` config block + `GET /_apps/{id}` CSP-sandboxed
HTML route (backend, mirrors theme-logo + actions).
2. Vue `/app/:id` host route + sandboxed-iframe component + `MessageChannel`
bridge exposing `rela.query*` (read) and `rela.create/update/delete/rename`
(write).
3. `OpRunApp` ACL Op (5-point checklist) + CSP origin allow-list config.
4. Bridge → `_action/{id}` wiring for registered Lua write actions.
5. Docs + e2e (an example app under `apps/`).
