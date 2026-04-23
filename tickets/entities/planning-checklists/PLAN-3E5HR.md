---
id: PLAN-3E5HR
type: planning-checklist
title: 'Planning: Refactor document links to app-relative + add Lua router/URL helpers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

**In scope:**

1. Replace `create://` / `edit://` rewriting in `internal/dataentry/document.go` with an app-relative link pass:
   - Recognize `href="/..."` links emitted from document HTML.
   - Verify each internal href against the route catalogue; if it matches a route marked `accepts_return_to`, append `return_to` (preserving existing query + hash). Otherwise leave untouched.
   - External (absolute `http(s)://`, `mailto:`, anchors) links are untouched.
2. **Register frontend routes as a first-class catalogue in Go**, in a new leaf package `internal/frontendroutes`. This is the single source of truth used by the Lua helper, the CLI subcommand, the link rewriter, and future surfaces. Frontend still declares its own Vue router (Vue-idiomatic); the two stay aligned via a Go parity test.
3. One Lua helper in `internal/lua` (Phoenix-verified-routes style):
   - `rela.url(path, params?)` — takes a literal path, verifies it against the catalogue (like Phoenix's `~p` sigil), and appends params as query. Unknown paths raise at call time. Arbitrary user-chosen param keys (including dotted keys like `prop.status`, `rel.parent`) go into the query. Lua-side param keys are snake_case; the helper maps them to the route's Vue-side param names where a route definition uses them.
   - Exposed in both read and write runtimes. Pure string builder — no capability gating.
4. Consumer-side interface in `internal/lua` (per CLAUDE.md rules) — `internal/lua` does **not** import `internal/frontendroutes`. It declares the minimum it needs:

   ```go
   // in internal/lua, defined at the call site
   type RouteCatalog interface {
       Match(path string) (MatchedRoute, bool)
       List() []Route  // for any introspection the runtime exposes
   }
   ```

Concrete implementation lives in `internal/frontendroutes` and is wired in via
`ReadDeps` / `WriteDeps`. Tests inject a fake catalogue.
5. `rela-server routes` CLI subcommand that prints the catalogue as a table (and `--format json`).
6. Documentation update in `GUIDE-data-entry.md`: drop the `create://` / `edit://` section; add a new "Linking from documents" section covering app-relative paths + `rela.url`.

**Out of scope:**

- Backcompat shim for `create://` / `edit://`. Strategy: detect them in the rewriter, log a warning naming the document, render as-is. CHANGELOG entry. One-shot migration tool is a potential follow-on ticket.
- Form-selection heuristics (`createFormForType` / `editFormForType`) — still used by sections/lists, untouched.
- REST API routes in the catalogue (already described via OpenAPI). `rela-server routes --api` noted as a follow-on.
- UI page for route browsing — follow-on ticket.
- Extending `return_to` honouring to entity/view/kanban pages in the Vue layer — follow-on ticket. For now, only the `form-*` routes carry `accepts_return_to: true`. The catalogue field exists so phase B is a one-line flag flip.
- Route permissions / auth.

**Acceptance Criteria:**

1. **Form links round-trip with `return_to`.**
   - *Test:* Markdown `[Edit](/form/full_ticket/TKT-001)` renders to an href with `return_to=%2F...%23tkt-001` appended. `[Create](/form/full_ticket?prop.status=open)` keeps the existing query and appends `return_to`.
2. **Non-form internal links unchanged.**
   - *Test:* `[List](/list/all_tasks)`, `[Detail](/entity/ticket/TKT-001)`, `[Search](/search?q=foo)` render with no query mutation. (The catalogue entry for these routes has `accepts_return_to: false`.)
3. **External and anchor links untouched.**
   - *Test:* `[docs](https://example.com)`, `[email](mailto:a@b.c)`, `[anchor](#section)` unchanged.
4. **Unknown internal paths are detected.**
   - *Test:* `[bogus](/nope/foo)` in markdown → link renders but the rewriter logs a warning naming the document and the unmatched path. Link is not mutated.
5. **Legacy schemes warned but passed through.**
   - *Test:* `[Edit](edit://ticket/TKT-001)` and `[Create](create://ticket)` in markdown → link renders as-is, warning logged, no rewrite.
6. **`rela.url` verifies paths.**
   - *Test:* `rela.url("/form/full_ticket/TKT-001")` returns `/form/full_ticket/TKT-001`. `rela.url("/form/full_ticket", {["prop.status"]="open", q="a b&c"})` returns `/form/full_ticket?prop.status=open&q=a%20b%26c` (keys sorted for determinism).
7. **`rela.url` rejects unknown paths.**
   - *Test:* `rela.url("/nope/foo")` raises a Lua error `unknown frontend route: /nope/foo`.
8. **`rela.url` preserves existing query/hash on the input path.**
   - *Test:* `rela.url("/form/full_ticket/TKT-001?x=1#section", {y="2"})` → `/form/full_ticket/TKT-001?x=1&y=2#section` (sorted query, hash preserved).
9. **Type/validation errors are loud.**
   - *Test:* `rela.url("/x", "not a table")` raises a Lua type error. A param value that isn't string/number raises an error naming the key.
10. **`rela-server routes` lists the catalogue.**
    - *Test:* Running `rela-server routes` prints a table with NAME, PATH, PARAMS (Lua names), RETURN_TO, NOTES columns. Output is stable-sorted. Exit 0. `rela-server routes --format json` emits `[]frontendroutes.Route` as JSON. Running `rela-server` with no args (or existing flags) still starts the server as before.
11. **Frontend ↔ Go catalogue parity.**
    - *Test:* A Go test parses `frontend/src/router/index.ts` (regex) and asserts every path + name present there is in the Go catalogue and vice versa. CI fails on drift with a clear "add/remove this route in X" message.
12. **Existing doc-link tests migrated.**
    - *Test:* `document_test.go:TestRewriteDocumentLinks` rewritten; `customLinkRegex`, `buildEditLink`, `buildCreateLink` deleted.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Path-verified routes — prior art:**
  - Phoenix `~p"/form/:id"` sigil — path *is* the API; validated against the router at compile time. Closest fit for our Lua-from-markdown use case: authors already think in paths, and we avoid inventing names.
  - Rails `*_path` / `*_url` — generated methods per route. Requires codegen.
  - Laravel `route('name', params)` / Django `reverse('name', args=...)` — runtime lookup by name. Works, but adds a name-vs-path debate we don't need.
  - **Choice:** Phoenix-style. One helper, `rela.url(path, params?)`, where the path is the literal route with concrete values. Runtime verification against the catalogue. If we later want a named-route sugar (for e.g. discoverability), it's additive.
- **Codebase patterns:**
  - Consumer-side interfaces per CLAUDE.md ("Define interfaces at the call site"). `internal/lua` and `internal/dataentry` each declare their own minimum interface — they have genuinely different needs (see Decisions below).
  - Lua binding registration: `internal/lua/runtime.go:509-592`. `rela.url` registers via `SetField(rela, "url", r.L.NewFunction(r.luaURL))` in `registerContextBindings` so both read and write runtimes expose it.
  - `ReadDeps` / `WriteDeps` (`internal/lua/deps.go`) are capability bundles split by **read vs. write of graph state**. The route catalogue is a stateless pure string builder, not a graph capability — it goes in via a `WithRouteCatalog(c)` runtime option, matching how `WithParams`, `WithSecrets`, and `WithDocumentMode` already work. If unset, `rela.url` raises "route catalogue not configured" (same pattern as `rela.cache` when the cache is absent).
- **Go-as-source-of-truth precedent:**
  - OpenAPI generator (`internal/openapi/generator.go:22-45`) generates spec from the Go metamodel, served via `/api/v1/_openapi.json`. The frontend doesn't consume the spec today, but the pattern of "Go owns the schema, TS mirrors it" is established.
  - For *frontend* routes, the other direction (TS owns, Go mirrors) is more natural because routes are tied to Vue component imports. A parity test is the cheapest fence.
- **TS → Go codegen options, evaluated:**
  - **(a) Regex-parse `router/index.ts` in a Go test.** ~10 lines. The file is 13 literal entries, no dynamic constructs. Fails CI on drift. **Chosen for v1.**
  - **(b) Vite plugin emits `frontend/dist/routes.json`.** ~50 lines. More robust to future router restructuring, but we don't need it yet. Noted as an upgrade path if the router grows complex.
  - **(c) Go → TS generator.** Requires refactoring the Vue router to import generated data. Fights the frontend's natural shape; rejected.
  - **(d) TS → Go at CI time + commit generated file.** High infra cost (Node in CI, generated file in repo). Rejected.
- **Reference implementations:**
  - Vue Router's runtime route table is introspectable but exposing it to Go adds coupling we don't need.
  - gorilla/mux and chi `Walk` — not applicable (these are HTTP server routes, not frontend).

**Decisions:**

- Lua API shape: **one flat function** `rela.url(path, params?)`, Phoenix-style path verification. No separate `rela.route(name, ...)` — path *is* the name.
- Snake_case on the Lua side (`entity_id`, not `entityId`) — matches the rest of the rela Lua surface. The `Route.Params` slice pairs the Vue-side name with the Lua-side alias (typed `Param` struct, not parallel slices).
- Catalogue package: **new leaf package `internal/frontendroutes`**, stdlib-only. Generic-enough name; leaves room for API routes later if we want one unified catalogue.
- `internal/frontendroutes` exposes **package-level functions**, not a constructed catalog value: `frontendroutes.All()`, `frontendroutes.Match(path)`. Less ceremony than `Catalog{}.Method()` for a zero-state type.
- **Two separate consumer-side interfaces**, one per consumer (per CLAUDE.md: each consumer declares the minimum it needs):
  - `lua.RouteCatalog` — `Has(path string) bool`. That's all `rela.url` needs today: verify existence, then build a query string.
  - `dataentry.routeMatcher` — `Match(path string) (MatchedRoute, bool)` with a local `MatchedRoute{AcceptsReturnTo bool}`. The rewriter needs more than existence; it needs the `AcceptsReturnTo` flag.
  - `frontendroutes.Match` and `frontendroutes.Has` structurally satisfy both. Neither consumer imports the other's interface, and neither needs to import `frontendroutes` for the interface definition (just for the concrete wiring at construction).
- Catalogue goes in via a `WithRouteCatalog(c lua.RouteCatalog)` runtime construction option, not on `ReadDeps`/`WriteDeps`. It's stateless config, not a graph capability.
- No `form://` sugar; no legacy `create://` / `edit://` shim (warned + passed through).
- `AcceptsReturnTo` is route metadata. Only `form-create` and `form-edit` start with `true`. Extending to entity/view is phase B, one-line flag flip + frontend wiring.
- TS → Go parity: regex-parse Go test (option a). Upgrade to a Vite plugin if the router grows complex.
- CLI subcommand lives on **`rela-server routes`**. The frontend catalogue is a rela-server concern (the regular `rela` CLI has no notion of frontend code). `cmd/rela-server/main.go` currently uses stdlib `flag` — we add minimal subcommand dispatch (or convert to cobra) to support `rela-server [serve] [flags]` vs. `rela-server routes [--format json]`. Prefer the smallest change: a `switch os.Args[1]` with the existing `flag.CommandLine` handling `serve` as the default.
- The rewriter accepts a `*slog.Logger` (not `slog.Default()` implicitly) for the warning logs — keeps it testable without stderr capture.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

### Step 1 — Route catalogue in a leaf package

New package `internal/frontendroutes`, stdlib-only:

```go
// Param pairs the Vue router's param name with the snake_case alias exposed to Lua.
// Typed so the two names can't silently desync via a slice-length bug.
type Param struct {
    Vue string // e.g. "entityId"
    Lua string // e.g. "entity_id"
}

// Route describes one frontend route.
type Route struct {
    Name            string  // "form-edit"
    Path            string  // "/form/:id/:entityId"
    Params          []Param // in segment order
    AcceptsReturnTo bool    // true for form-* routes; extends in phase B
    Notes           string  // optional human-readable hint
}

// MatchedRoute is what Match returns — a Route plus the param values extracted from the literal path.
type MatchedRoute struct {
    Route  Route
    Values map[string]string // Vue-name → value (e.g. {"id":"full_ticket","entityId":"TKT-001"})
}

// Package-level API (no constructed catalog value; the zero value is the API).

// All returns all known routes (stable-sorted by name).
func All() []Route { ... }

// Has reports whether a literal path matches any known route pattern.
func Has(path string) bool { ... }

// Match finds the route whose pattern matches a literal path
// (e.g. "/form/full_ticket/TKT-001" matches "/form/:id/:entityId").
func Match(path string) (MatchedRoute, bool) { ... }

// routes is the private catalogue.
var routes = []Route{
    {Name: "dashboard",   Path: "/dashboard"},
    {Name: "list",        Path: "/list/:id",                 Params: []Param{{"id","id"}}},
    {Name: "form-create", Path: "/form/:id",                 Params: []Param{{"id","form_id"}},                                AcceptsReturnTo: true, Notes: "id = form id"},
    {Name: "form-edit",   Path: "/form/:id/:entityId",       Params: []Param{{"id","form_id"}, {"entityId","entity_id"}},      AcceptsReturnTo: true, Notes: "id = form id; entityId = entity being edited"},
    {Name: "entity",      Path: "/entity/:type/:id",         Params: []Param{{"type","type"}, {"id","id"}}},
    {Name: "view",        Path: "/view/:id/:entityId",       Params: []Param{{"id","id"}, {"entityId","entity_id"}}},
    {Name: "kanban",      Path: "/kanban/:id",               Params: []Param{{"id","id"}}},
    {Name: "search",      Path: "/search"},
    {Name: "analyze",     Path: "/analyze"},
    {Name: "settings",    Path: "/settings"},
    {Name: "conflicts",   Path: "/conflicts"},
    {Name: "document",    Path: "/document/:name/:entityId", Params: []Param{{"name","name"}, {"entityId","entity_id"}}},
}
```

### Step 2 — Consumer-side interface in `internal/lua`

`internal/lua/routes.go` (new):

```go
// RouteCatalog is the minimum surface rela.url needs:
// verify whether a path matches a known route pattern.
//
// Defined at the call site per CLAUDE.md. Satisfied structurally by
// frontendroutes.Has (passed as a RouteCatalogFunc — see wiring below).
type RouteCatalog interface {
    Has(path string) bool
}

// RouteCatalogFunc adapts a plain function into a RouteCatalog.
// Lets callers pass frontendroutes.Has without defining a wrapper type.
type RouteCatalogFunc func(path string) bool

func (f RouteCatalogFunc) Has(path string) bool { return f(path) }
```

Runtime option (matches `WithParams`, `WithSecrets`, `WithDocumentMode`):

```go
// WithRouteCatalog wires a catalogue into the runtime; rela.url requires this.
// If unset, rela.url is absent from the rela.* table (same pattern as rela.cache).
func WithRouteCatalog(c RouteCatalog) RuntimeOption { ... }
```

Wiring at runtime-construction sites:
`WithRouteCatalog(lua.RouteCatalogFunc(frontendroutes.Has))`. Tests inject a
fake func.

### Step 3 — Lua binding

`internal/lua/urls.go` (new):

```go
// rela.url(path, params?) → string
//
// Verifies path against the route catalogue. Returns path (+ optional query).
// Raises if the path doesn't match any known route, or if params has invalid types.
func (r *Runtime) luaURL(ls *lua.LState) int {
    path := ls.CheckString(1)
    base, rawQuery, fragment := splitPathQueryFragment(path)
    if !r.routes.Has(base) {
        ls.RaiseError("unknown frontend route: %s", base)
        return 0
    }
    merged := mergeQuery(rawQuery, readParams(ls, 2))
    ls.Push(lua.LString(buildURL(base, merged, fragment)))
    return 1
}
```

Registered in `registerContextBindings` (`runtime.go:557`) only when a catalogue
was configured — absent by default, matching `rela.cache`:

```go
if r.routes != nil {
    r.L.SetField(rela, "url", r.L.NewFunction(r.luaURL))
}
```

### Step 4 — Link rewriter refactor

`internal/dataentry/document.go`:

- Delete `customLinkRegex`, `RewriteDocumentLinks`, `rewriteDocumentLink`, `buildEditLink`, `buildCreateLink`.
- Define consumer-side interface in the same file (CLAUDE.md: call-site interface):

  ```go
  // routeMatcher is what the rewriter needs: match a path against the route
  // catalogue and report whether the matched route honours return_to.
  // Satisfied structurally by frontendroutes.Match (via a local adapter).
  type routeMatcher interface {
      Match(path string) (matchedRoute, bool)
  }

  type matchedRoute struct {
      AcceptsReturnTo bool
  }
  ```

- Replace with `RewriteDocumentLinks(html, returnPath string, m routeMatcher, log *slog.Logger)`:
  - Regex-scan `href="..."`.
  - Skip external schemes (`http`, `https`, `mailto`, empty, anchor-only).
  - For internal hrefs, call `m.Match(basePath)`.
    - Unmatched → `log.Warn("document link has no matching route", "href", ...)`, leave link as-is.
    - Matched + `AcceptsReturnTo` → inject `return_to` preserving existing query + hash; for edit-form routes, `return_to` value gets `#<lowercased entity id>` appended (existing behaviour).
    - Matched + not `AcceptsReturnTo` → leave untouched.
  - Detect `create://` / `edit://` prefixes → `log.Warn("legacy document-link scheme; rewrite to app-relative path", ...)`, leave href as-is.

Package `dataentry` also provides a tiny adapter that maps
`frontendroutes.Match` to its local `matchedRoute`, since the return types
differ:

```go
func matchFrontendRoute(path string) (matchedRoute, bool) {
    m, ok := frontendroutes.Match(path)
    if !ok { return matchedRoute{}, false }
    return matchedRoute{AcceptsReturnTo: m.Route.AcceptsReturnTo}, true
}
```

Wrapped as a `routeMatcherFunc` adapter (same pattern as `RouteCatalogFunc`).

### Step 5 — CLI subcommand

`rela-server routes` in `cmd/rela-server/`. The frontend route catalogue belongs
to `rela-server` (the regular `rela` CLI has no notion of frontend code).

- `rela-server routes` → prints `NAME | PATH | PARAMS (Lua names) | RETURN_TO | NOTES`.
- `rela-server routes --format json` emits `[]frontendroutes.Route` directly.
- No network calls; reads `frontendroutes.All()`.

Minimal structural change to `main.go`: keep the existing `flag`-based server
behaviour when invoked as `rela-server [flags]` (no subcommand); dispatch on
`os.Args[1] == "routes"` before calling `flag.Parse()`. If subcommand surface
grows later, convert to cobra then.

### Step 6 — Frontend parity test

Lives in a test-only package `internal/frontendparity` (its whole job is
cross-boundary checks against `frontend/`). Keeps `internal/frontendroutes` a
true stdlib-only leaf.

- Open `frontend/src/router/index.ts` (path relative to `internal/frontendparity`), regex-extract `path: '(...)'` and `name: '(...)'` pairs.
- Compare the extracted set to `frontendroutes.All()`. Fail with clear messages:
  - `Route present in frontend but missing from catalogue: name=... path=...`
  - `Route in catalogue but missing from frontend: name=... path=...`
- If the regex fails to match any routes, fail with "update parity test to match new router shape".

**Files to modify:**

- `internal/frontendroutes/routes.go` (new) — catalogue + `All` / `Has` / `Match`, package-level
- `internal/frontendroutes/routes_test.go` (new) — unit tests
- `internal/frontendparity/parity_test.go` (new) — TS parity, test-only package
- `internal/lua/routes.go` (new) — consumer-side `RouteCatalog` interface + `RouteCatalogFunc`
- `internal/lua/options.go` (or wherever `WithParams`/`WithSecrets` live) — add `WithRouteCatalog`
- `internal/lua/urls.go` (new) — `rela.url` binding
- `internal/lua/urls_test.go` (new) — with fake `RouteCatalogFunc`
- `internal/lua/runtime.go` — register `rela.url` conditionally in `registerContextBindings`
- `internal/dataentry/document.go` — rewrite link handling; local `routeMatcher` interface + adapter for `frontendroutes.Match`; accept `*slog.Logger`
- `internal/dataentry/document_test.go` — rewritten tests
- `internal/dataentry/app.go` + `cmd/rela-server/main.go` + any other Lua runtime construction sites — wire `WithRouteCatalog(lua.RouteCatalogFunc(frontendroutes.Has))` (to be enumerated at impl start by grepping for `lua.NewReader` / `lua.NewWriter`)
- `cmd/rela-server/main.go` — add `routes` subcommand dispatch; keep existing server behaviour as default
- `cmd/rela-server/routes.go` (new) — `rela-server routes` implementation (table + JSON output)
- `docs-project/entities/guides/GUIDE-data-entry.md` — drop old scheme, document new helper
- `CHANGELOG` (if one exists) — announce removal of `create://` / `edit://`

**Alternatives considered / rejected:**

- **Two helpers `rela.url` + `rela.route(name, params)`.** Rejected — Phoenix-style path verification folds them into one, and the path *is* the human-readable reference. If named-route sugar becomes valuable later (e.g. for UI autocomplete), it's additive.
- **Name catalogue `SPARoutes` / `internal/sparoutes`.** Rejected — "SPA" is an implementation detail; `frontendroutes` is the concept.
- **Put catalogue in `internal/dataentry`.** Rejected — `internal/lua` must not import `internal/dataentry` (per CLAUDE.md, and avoids a potential cycle). Leaf package is cleaner.
- **Add `Routes RouteCatalog` to `ReadDeps`/`WriteDeps`.** Rejected after review — those bundles are specifically about graph capabilities (read vs. write of entity/relation state). A stateless string-verifier isn't a capability; it's runtime config. Goes in via `WithRouteCatalog` option instead.
- **Single `RouteCatalog` interface shared between Lua and the rewriter.** Rejected — they have different needs. Lua wants existence (`Has`), the rewriter wants the `AcceptsReturnTo` flag. Two call-site interfaces keep each consumer minimal (CLAUDE.md).
- **Parallel `VueParams []string` + `LuaParams []string` slices on `Route`.** Rejected — they can desync via a slice-length bug. Replaced with `[]Param{Vue, Lua}` struct slice.
- **Constructed `Catalog{}` value with methods.** Rejected — zero-state type, prefer package-level `frontendroutes.All()` / `Has()` / `Match()`. Less ceremony, clearer single-instance nature.
- **`rela routes` in `internal/cli`.** Rejected — the regular `rela` CLI has no notion of frontend code. The catalogue is a rela-server concern; subcommand belongs in `cmd/rela-server/`. Keeping the main-binary stdlib-`flag` shape and dispatching on `os.Args[1]` is the smallest viable change; cobra conversion can come later if more subcommands land.
- **Auto-inject `return_to` on every internal link.** Rejected for v1 — the Vue layer only honours it on forms today. Adding it to other routes without the frontend handling it is a bad UX (clutter + false promises). Phase B can flip the flag once `EntityView`, `ViewView`, etc. honour `return_to`.
- **Vite plugin for routes.json.** Noted as a future upgrade path. Regex parse is cheaper for v1.
- **Runtime route discovery via `/api/v1/_routes`.** Rejected — adds a server round-trip for a static list.
- **Generate Rails-style `*_path` Lua functions.** Rejected — no codegen today, and the generator cost isn't justified for <15 routes.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Document markdown (from Lua scripts / shell commands).** Already untrusted; goldmark sanitizes. The new rewriter only inspects `href` attribute values; malformed URLs are left as-is (same as today).
- **Lua path arg to `rela.url`.** Allowlist: must match a route pattern in the catalogue. Unknown paths raise at call time.
- **Lua params table to `rela.url`.** Values: URL-escaped via `url.QueryEscape`. Path-segment substitutions (if any, for named-route sugar later) use `url.PathEscape`. Keys: reject those containing `&`, `=`, or whitespace to avoid query injection. Non-string/number values raise a typed error.
- **`return_to` value in rewriter.** URL-encoded. Continues to be the document's own path; not user input from a different origin.
- **Legacy scheme warning log.** Logs the document ID and the href; does not log full markdown content.

**Security-Sensitive Operations:**

- None introduced. `rela.url` produces strings; no filesystem or network access. The CLI subcommand reads a static slice. No auth surface changes.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (mapping AC → test)

- AC1, AC2, AC3, AC4, AC5 → `internal/dataentry/document_test.go` table-driven tests on the rewriter, injecting a concrete `frontendroutes.Catalog{}`.
- AC6, AC7, AC8, AC9 → `internal/lua/urls_test.go`. Uses a fake `RouteCatalog` implementation to isolate the binding logic.
- AC10 → `internal/cli/routes_test.go` (or `cmd/rela-server/cmd/routes_test.go`) — invoke the subcommand, assert stdout in both formats.
- AC11 → `internal/frontendroutes/parity_test.go`.
- AC12 → existing doc tests updated; CI green.

**Integration test:** end-to-end document render with a Lua script calling
`rela.url(...)`, assert the rendered HTML contains the expected path +
`return_to`. Uses `frontendroutes.Has` via `WithRouteCatalog` through the real
`lua.Runtime` construction path, and the real `frontendroutes.Match` in the
rewriter, so it exercises the full wire-up.

**Logger assertions:** the rewriter's warnings are captured via a test
`*slog.Logger` backed by a `slog.NewJSONHandler(buf, ...)`. Tests assert the
warning is emitted exactly once per unmatched/legacy href, with the expected
attributes.

**Edge Cases:**

- Empty params table: `rela.url("/x", {})` → `/x` (no trailing `?`).
- `nil` second arg: `rela.url("/x")` → `/x`.
- Path with existing query: `rela.url("/x?a=1", {b="2"})` → `/x?a=1&b=2` (sorted).
- Path with fragment: `rela.url("/x#s", {a="1"})` → `/x?a=1#s`.
- Unicode in values: UTF-8 percent-encoded.
- `rela.url` with root-only path `"/"` — matches `dashboard` via the redirect? Decision at impl: `/` is not in the catalogue (it's a redirect, not a real route); we require `/dashboard`. Document this.
- `rela.url("/form/full_ticket/TKT%2F001")` — already-encoded segments pass through unchanged.
- Query key ordering: deterministic (sorted).
- Author writes `return_to` in markdown (e.g. `[x](/form/foo?return_to=/bar)`) → rewriter overwrites with the document-derived `return_to`. Documented: `return_to` is reserved.
- Rewriter encounters an href without a leading `/` (e.g. `foo/bar`) → treated as external, untouched.
- Path length: no limit; `url.Parse` handles it.
- Concurrent document renders: `Catalog` has no state, safe for concurrent use.

**Negative Tests:**

- Unknown path: `rela.url("/nope/foo")` → `unknown frontend route: /nope/foo`.
- Non-table second arg: `rela.url("/x", "not a table")` → Lua type error.
- Function as param value: `rela.url("/x", {a=function() end})` → typed error naming key `a`.
- Injection key: `rela.url("/x", {["a&b"]="1"})` → error rejecting key `a&b`.
- Legacy `create://` / `edit://` in markdown — no rewrite; warning logged.
- Author path with typo (`/form/full_tickt/TKT-001`) — matches pattern (because `/form/:id/:entityId` is a template), so passes. Typo bugs aren't this ticket's problem.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Silent breakage of existing `create://` / `edit://` links.**
  - *Mitigation:* CHANGELOG entry + warn log at render time (named document + href) + grep the in-tree prototypes, docs-project, and tickets/ and migrate them in this PR. Downstream users see the warning and migrate.
- **Path verification being too strict.**
  - *Mitigation:* `Match` compares against the pattern, not the literal — `/form/full_ticket/TKT-001` matches `/form/:id/:entityId`. Document authors don't need the catalogue to know every form ID.
- **Parity test flakiness parsing TS.**
  - *Mitigation:* Anchor regex to the `routes: RouteRecordRaw[] = [ ... ]` block; keep regex narrow (`path: '([^']+)'`, `name: '([^']+)'`). Test fails loudly with a "update parity test" message if the file structure changes.
- **Import wiring sprawl.**
  - *Mitigation:* Every Lua runtime construction site needs `WithRouteCatalog(lua.RouteCatalogFunc(frontendroutes.Has))`. Enumerate at impl start by grepping for `lua.NewReader` / `lua.NewWriter`. If a site genuinely doesn't need `rela.url`, leave it unset — the binding is conditionally registered so it's absent rather than broken.
- **Lua/Vue param-name drift.**
  - *Mitigation:* The catalogue carries both names; parity test checks `VueParams` against what the TS file exposes. Lua-side rename is local to the catalogue.

**Effort:** `m` — one new leaf package, one Lua binding, one CLI subcommand, one
rewriter refactor, one parity test. ~1–2 days.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide — `docs-project/entities/guides/GUIDE-data-entry.md` section on documents (lines ~1794-1939): remove the `create://` / `edit://` scheme reference table; add "Linking from documents" with app-relative paths + `rela.url` examples.
- [x] CLI help text — `rela-server routes --help`.
- [x] CLAUDE.md — mention the new `rela.url` helper and the `frontendroutes` catalogue under architectural patterns.
- [x] ~~README.md~~ (N/A: no project-level surface change.)
- [x] ~~API docs~~ (N/A: no HTTP surface change.)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (Deferred: awaiting user approval of plan first; will run before transitioning to in-progress if requested.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A until design review is run.)

**Design Review Findings:** <!-- Populated after /design-review -->
