---
id: PLAN-O8KMBQ
type: planning-checklist
title: 'Planning: Remote MCP: serve MCP over Streamable HTTP from rela-server, per-request principal + ACL'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `rela mcp` is stdio-only and wires `acl.NopACL{}` + a single
`principal.SystemUser()` at construction. Both are correct *for stdio* — the
filesystem is the trust boundary, so a tool-surface gate would defend nothing
(`internal/cli/mcp_wiring.go`). Serving MCP from a `rela-server` deploy inverts
that: the caller has no filesystem access, is not one trusted local user, and is
not implicitly authorized. So this is a **security change wearing a transport
change's clothes**, and the ACL work — not the HTTP listener — is the substance.

**Scope:** as recorded on TKT-UIR41P. Summary of the three settled decisions:

1. Migrate `internal/mcp` to official `modelcontextprotocol/go-sdk` v1.7.0
   (protocol `2026-07-28`).
2. Mount Streamable HTTP at `/api/v1/_mcp`, inside the existing middleware
   chain; add RFC 9728 Protected Resource Metadata + `WWW-Authenticate`.
3. Lua tools (`lua_eval`/`lua_run`/`lua_list`) are NOT registered remotely.
   Deferred to its own ticket, where BOTH read and write deps get rewired.

**Out:** stdio removal, multi-tenancy, TKT-G3PPD's tool-list intersection,
OAuth AS duties (rela stays a resource server).

**Acceptance criteria → test scenarios:** see the Test Plan section below;
each of AC1–AC9 is mapped to a named test there.

## Research

- [x] ~~Run `/research` to create a structured research doc~~ (N/A: two
  research sweeps were run during ticket creation and their findings are
  recorded on TKT-UIR41P — the SDK/spec comparison and the server-wiring map.
  A RES- entity would duplicate the ticket body.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** <!-- Link RES-xxxx if created, or N/A for small changes -->

**Libraries considered** (verified from module proxy + module cache, not docs):

| Module | Version | `LATEST_PROTOCOL_VERSION` | Verdict |
|---|---|---|---|
| `mark3labs/mcp-go` | v0.57.0 (current) | `2025-11-25` | status quo |
| `mark3labs/mcp-go` | v0.58.0 (stable) | `2025-11-25` | no spec gain |
| `mark3labs/mcp-go` | v1.0.0-beta.1 | `2026-07-28` | rejected: 1-day-old beta in an auth path |
| `modelcontextprotocol/go-sdk` | **v1.7.0** | `2026-07-28` | **CHOSEN** |

Rejected staying on `mark3labs` because no *stable* release speaks 2026-07-28;
building a brand-new network surface on an already-superseded revision means
doing the transport work twice.

**Similar patterns in codebase (reuse, don't reinvent):**

- `internal/dataentry/app.go:445` `gatedScriptReader` and
  `internal/appbuild/appbuild.go:250` `scriptEntityReader` — the canonical
  3-step ACL composition (`NewDeclarativeGate` → `NewPolicyReader` →
  `NewScriptReader`), including the fail-closed `DenyReader` on construction
  error and the `Unrestricted` NopACL path. MCP's wiring should mirror this.
- `internal/appbuild/appbuild.go:285` `scriptTracer` — the tracer analog.
- `internal/dataentry/visiblereader.go:35` `visibleReader` — the gate-first
  `getVisible` pattern that makes hidden and absent indistinguishable, and
  `filterVisible`'s batched, fail-closed drop. MCP cannot import it
  (`dataentry` is not in mcp's `mayDependOn`), but it is the shape to copy.
- `internal/dataentry/readgate.go:48` `SearchScope` — how a `*acl.Request`
  becomes the `map[string]search.TypeScope` that `VisibleSearcher` needs.
- `internal/dataentry/export.go:58` — precedent for building a
  `visibility.PolicyReader` over a ctx-resolving row gate outside the main App
  read path.

**Key architectural finding (drives the whole approach):**
`internal/visibility` resolves the principal **from ctx at call time**
(`adapters.go:41`, `principal.From(ctx)`) rather than capturing it at
construction. That is why one wrapped instance can serve every request — and it
is exactly what makes a per-request-principal MCP feasible without rebuilding
deps per call.

**Prior art in the graph:** DEC-RG878 (transport-layer intersection,
default-deny on writes), TKT-G3PPD (tool-list filtering — sibling),
DEC-O59WM4 (identity-bound Lua read deps), DEC-ZBI39P (visibility decorators at
the wiring site), TKT-POT9GQ (SSE frames carry no entity ids).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Blast radius (measured)

SDK coupling is **entirely contained in `internal/mcp`** — 16 files, zero
imports of `mark3labs` anywhere in `cmd/` or other packages. 34 handlers,
2913 prod LOC + 2444 test LOC. 117 result-helper call sites but only **two**
distinct helpers (`NewToolResultError` ×74, `NewToolResultText` ×43), so the
bulk of the diff is mechanical behind two local shims.

### go-sdk v1.7.0 API map (verified from module cache)

| Concern | mark3labs v0.57.0 | go-sdk v1.7.0 |
|---|---|---|
| Construct | `server.NewMCPServer(name, ver, opts...)` | `mcp.NewServer(*Implementation, *ServerOptions)` (`server.go:182`) |
| Instructions | `server.WithInstructions(...)` | `ServerOptions.Instructions` (`server.go:71`) |
| Register tool | `s.AddTool(mcp.NewTool(...), h)` | `mcp.AddTool[In,Out](s, *Tool, h)` (`server.go:561`) |
| Args | `req.RequireString("id")` | typed `In` struct; schema reflected from `jsonschema:"..."` tags |
| Tool error | `mcp.NewToolResultError(msg)` | generic handler: `return nil, nil, err` → `IsError:true` (`tool.go:48`) |
| Protocol error | n/a | raw `Server.AddTool` handler, or return `*jsonrpc.Error` |
| Middleware | `WithToolHandlerMiddleware` | `AddReceivingMiddleware(Middleware)` (`server.go:1770`), method-level |
| HTTP | `NewStreamableHTTPServer` | `NewStreamableHTTPHandler(getServer func(*http.Request) *Server, opts)` (`streamable.go:232`) — is an `http.Handler` |
| Stdio | `server.ServeStdio` | `s.Run(ctx, &mcp.StdioTransport{})` (`server.go:1285`) |
| `server/discover` | n/a | **automatic** (`server.go:884`) — nothing to implement |

**Per-request principal — the key seam.** There is no `WithHTTPContextFunc`
analog. Two mechanisms exist and the second is the one we want:
`req.Extra.Header` (`shared.go:601`) exposes headers but not the `*http.Request`;
**`getServer func(*http.Request) *Server`** is called per request in stateless
mode and receives the live request. Since our middleware chain has already
stamped the principal and attached the `*acl.Request` onto `r.Context()` by the
time the handler runs, ordinary `http.Handler` context propagation suffices —
the request context flows into handler contexts
(`streamable_example_test.go:44`). So the existing ctx-based `internal/visibility`
wrappers work unmodified. Pair with `ServerOptions.SchemaCache`
(`server.go:148`) so per-request `Server` construction does not re-reflect every
schema.

### DESIGN CHANGE forced by the SDK: the watcher notification

Verified in source, and it contradicts an assumption in the ticket body:

- `Stateless` mode returns **405 on GET and DELETE** (`streamable.go:380-386`),
  rejects all server→client *requests*, and delivers notifications **only
  within an in-flight request** (`streamable.go:130-146`).
- Independently, for protocol ≥ `2026-07-28`, `list_changed` reaches a session
  **only if it holds an active `subscriptions/listen` stream** that opted in
  (`server.go:733-760`).
- And `2026-07-28` is *only* reachable via `server/discover`; the legacy
  `initialize` path **caps at `2025-11-25`** (`shared.go:75-78`).

**Consequence:** the stdio watcher's `SendNotificationToAllClients(
resources/list_changed)` has **no stateless-HTTP equivalent**. Recommendation:
do **not** wire the store-event bridge to the remote transport at all in this
ticket. Remote clients re-list on demand; a dropped notification is not a
correctness bug. This also sidesteps the `TKT-POT9GQ` concern entirely, since we
emit nothing. Keep the watcher on stdio only, and record the limitation in docs.

### Technical approach — two PRs

**PR 1 — SDK migration, stdio only, zero behavior change.**
1. `go.mod`: add `modelcontextprotocol/go-sdk v1.7.0`; drop `mark3labs/mcp-go`.
2. `.go-arch-lint.yml:148`: repoint the `mcpgo` vendor alias.
3. Port `server.go` construction; `Instructions` moves to `ServerOptions`.
4. Port 34 handlers. Per handler: define an `In` struct with `json`/`jsonschema`
   tags replacing the `RequireString`/`GetString`/`GetInt` plucking (17+12+7
   call sites), and swap the two result helpers for local shims so the error
   semantics stay identical.
5. Replace `principalMiddleware` with `AddReceivingMiddleware` branching on
   `method == "tools/call"` — same single choke point, so a new tool still
   inherits the stamp automatically (preserve that CLAUDE.md property).
6. Port `dispatch_test.go`: it drives raw JSON-RPC through
   `s.mcp.HandleMessage`, which is SDK-specific. Rebuild it on
   `mcp.NewInMemoryTransports()` (`transport.go:157`) — this is the harness that
   proves AC #6, so it must survive.
7. `rela mcp` runs `s.Run(ctx, &mcp.StdioTransport{})`.

**PR 2 — remote transport + ACL.**
8. Narrow `mcp.Deps` read fields from the wide `store.Store` to consumer-side
   read interfaces (`GetEntity`/`ListEntities`/`ListRelations`/counts — the
   measured set), so the wiring site can inject either raw (stdio) or
   visibility-wrapped (HTTP) implementations. `mcp` must NOT import `acl` or
   `visibility` — neither is in its `mayDependOn` (`.go-arch-lint.yml:373`), and
   that stays true.
9. Wiring site builds the wrapped readers with the existing 3-step composition
   (mirroring `appbuild.go:250`/`:285`), including fail-closed `DenyReader`.
10. Fix `convert.go:106`/`:118` (`buildStoreRelations`) to gate neighbor
    lookups — otherwise `show_entity` leaks hidden neighbors' ids and titles.
11. `search_entities`: supply a per-request `map[string]search.TypeScope` via a
    wiring adapter (`SearchScope` equivalent), since `VisibleSearcher` does not
    read ctx.
12. Mount at `/api/v1/_mcp` in `registerAPIV1Routes` (`api_v1.go:82`) — must be
    an explicit pattern, since `/api/v1/` is a catch-all to
    `handleV1DynamicRoutes`. Off unless enabled.
13. RFC 9728 metadata endpoint + `WWW-Authenticate` on 401.
14. Register the tool set WITHOUT `lua_*` on the remote server.

**Alternatives rejected:** (a) wrapping the whole `store.Store` — it is a
10-interface composite (`store.go:170`), far too wide; consumer-side narrow
interfaces are what CLAUDE.md prescribes. (b) Importing `dataentry.visibleReader`
— forbidden by arch-lint and correctly so. (c) Stateful HTTP mode to keep
notifications — would forfeit `2026-07-28`, the whole point of the migration.

**Files to modify:** `go.mod`, `.go-arch-lint.yml`, all 16 files in
`internal/mcp/`, `internal/cli/mcp_wiring.go`, `internal/cli/mcp.go`,
`internal/dataentry/api_v1.go` (+ a new `mcp_handler.go` there or in wiring),
`cmd/rela-server/main.go` (flag + wiring), `docs-project/entities/` (source for
`docs/mcp-server.md`), `docs/server-security.md`, `docs/acl-security.md:743`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**This is the load-bearing section of the ticket.** The transport is routine;
the security posture is the deliverable.

**Input Sources & Validation:**

| Source | Validation | On invalid |
|---|---|---|
| `Authorization: Bearer` JWT | `jwtauth.Verifier`: ES256 only, pinned `iss`+`aud`, `WithExpirationRequired`, https-only JWKS | 401 via `writeV1Error` — `application/problem+json`, NOT an empty body (corrected in review); sampled log |
| JSON-RPC body (tool args) | Per-tool schema by the SDK; then existing `resolveType`/`trimID`/`isSafePathSegment` helpers | tool-result error, not a protocol error |
| `MCP-Protocol-Version` header | must equal the `_meta` value (2026-07-28 rule) | `400` + JSON-RPC `-32020` |
| `Origin` / `Host` | existing `requireSameOrigin` / `requireLocalHost` | 403 JSON `{error, reason}` |
| Entity/relation IDs | existing path-segment validation | indistinguishable not-found |

**Security-sensitive operations and how they're protected:**

1. **Read gating (the core defect).** Today MCP read handlers call
   `s.deps.Store.GetEntity` directly — e.g. `tools_entity.go:86` in
   `handleShowEntity` — with **no** row gate and **no** `visible:` redaction.
   `docs/acl-security.md:743` records this as an accepted gap justified *only*
   by "the MCP server is local-only (stdio)". Remote MCP voids that.
   Surface to rewire: **42 raw `deps.Store`/`deps.Tracer`/`deps.Searcher` sites
   across 8 files** — `tools_entity.go` (7), `tools_analysis.go` (7),
   `prompts.go` (7), `tools_trace.go` (4), `resources.go` (2),
   `tools_relation.go` (2), `tools_schema.go` (2), `tools_export.go` (1).
   **Corrected during design review** — an earlier count of ~25 scoped only the
   entity/relation tools and missed three whole surfaces (see D1–D3 below).
2. **Neighbor-title leak (found while planning — easy to miss).**
   `convert.go:106` and `:118` (`buildStoreRelations`) fetch **every** neighbor's
   title from the raw store. A correctly-gated `show_entity` would *still* leak
   hidden neighbors' ids and titles through the `relations` block. Needs the
   same neighbor-visibility gate `dataentry` applies for list-table export
   (CLAUDE.md, `visibleRelationIDs`). **Must be covered by a test.**
3. **Search asymmetry.** Unlike the reader/tracer, `search.VisibleSearcher`
   does NOT resolve identity from ctx — `SearchVisible(ctx, q, scope)`
   (`internal/search/types.go:146`) takes an explicit
   `map[string]search.TypeScope`. So `search_entities` needs a scope computed
   per request (the `readGate.SearchScope` equivalent), supplied via a wiring
   adapter since `dataentry.readGate` is unexported and off-limits to `mcp`.
4. **Write attribution.** Writes already pass the `entitymanager` ACL gate, but
   must now run under the *request's* principal, not `SystemUser()`, or the
   audit log attributes every remote write to `system`.
5. **Role forgery.** `principal.Verified` is the only constructor that populates
   the unexported `orgID/orgSlug/roles`. The MCP principal MUST come from
   `verifiedPrincipal` (`router.go:554`) after signature verification — never a
   composite literal.
6. **Lua excluded.** Sandbox is real (`SkipOpenLibs: true`; no
   `io`/`os`/`debug`/`package`), but `LuaWriteDeps` carries *unrestricted reads*
   (`appbuild.go:200`,`:308`), making `lua_eval` an ACL-bypassing read
   primitive. Not registered remotely; test-pinned.
7. **Error handling.** Denials must stay indistinguishable from not-found for
   **entity ids** (row-level secrecy), while a **config-declared capability**
   may name the missing permission (CLAUDE.md: "the configuration is not a
   secret"). Do not contort to hide tool names or property names.

**Explicitly NOT claimed:** tenant isolation. `principal.OrgID()` is audit
attribution only — nothing in `internal/acl` evaluates it (TKT-RP3X3Q), so a
principal in org A sees everything their role grants in every org. Docs must not
imply otherwise.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios (AC → test):**

| AC | Scenario |
|---|---|
| 1 | `TestRemoteMCP_DiscoverListCall`: `httptest` server over the real router; `server/discover` → `tools/list` → `show_entity`, asserting a JSON result. |
| 2 | `TestRemoteMCP_DisabledByDefault`: router built without the flag → POST `/api/v1/_mcp` is 404. Guards the upgrade path. |
| 3 | `TestRemoteMCP_ACLScopedReads` (the important one): one fixture, two principals with different `acl.yaml` grants, same endpoint. Asserts (a) `list_entities` returns disjoint sets; (b) `show_entity` on a denied id is **indistinguishable** from a nonexistent id (byte-identical response); (c) a `visible:`-restricted property is absent for the ungranted principal; (d) **`show_entity` on a *permitted* entity does not leak hidden neighbours' ids/titles** via the relations block (the `buildStoreRelations` leak); (e) `search_entities` honours the per-request scope. |
| 4 | `TestRemoteMCP_AuditAttribution`: write via remote MCP, assert the audit record carries the request's user (not `system`/`unknown`) and `Tool == mcp`. |
| 5 | `TestRemoteMCP_UnauthenticatedDenied`: JWT gate on, no/invalid assertion → 401; and a spoofed principal header does NOT authenticate (no downgrade). |
| 6 | **The stdio-parity harness.** Port `dispatch_test.go` onto `mcp.NewInMemoryTransports()` and keep the existing golden assertions for all 26 tools: name, arg decoding, result shape. This is the regression net for the migration; it must pass unchanged in PR 1. |
| 7 | `TestRemoteMCP_NoLuaTools`: remote `tools/list` contains no `lua_eval`/`lua_run`/`lua_list`; the stdio server's list still does. |
| 8 | `TestRemoteMCP_ProtocolVersion`: `server/discover` negotiates `2026-07-28`. Regression-guards the `initialize`-caps-at-`2025-11-25` trap (`shared.go:75`). |
| 9 | `TestRemoteMCP_ProtectedResourceMetadata`: 401 carries `WWW-Authenticate: Bearer … resource_metadata=…`; the well-known endpoint returns valid RFC 9728 JSON. |

**Integration approach:** AC1–5 and 7–9 run against `App.NewRouter()` via
`httptest`, so the real middleware chain (`stampAuditPrincipal` →
`requireVerifiedJWT` → `attachACLRequest` → host/origin) is exercised — not a
hand-assembled handler. This matches the existing `ServeHTTP` test convention
from TKT-TLQ94B.

**Edge cases:**

- **GET/DELETE on `/api/v1/_mcp`** → 405 + `Allow: POST` (stateless mode,
  `streamable.go:380`). Assert it, so a future stateful flip is noticed.
- **`MCP-Protocol-Version` header ≠ `_meta` value** → 400 + `-32020`.
- Missing `Accept: text/event-stream` → SDK rejects (`streamable.go:388`).
- Body > 4 MiB default (`MaxRequestBodyBytes`) — pick a deliberate limit.
- Gate **error** (not denial) mid-list → fail closed, drop the type, log loud.
- Entity hidden *between* the gate probe and the read → still no leak, since
  the gate runs first.
- Concurrent requests: each gets its own `*acl.Request` (not goroutine-safe by
  contract, `request.go:47`) — assert with `-race` and parallel calls.
- Unicode / oversized tool arguments — schema validation rejects before dispatch.

**Negative tests:** unauthenticated (401); authenticated-but-unauthorized read
(indistinguishable 404-equivalent); authenticated-but-unauthorized write
(`*acl.ForbiddenError` surfaced as a tool error, not a 500); malformed JSON-RPC;
unknown tool name; `lua_eval` called anyway on the remote server (must be
"unknown tool", never a dispatch).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Effort: XL.** Migration (~5.4k LOC incl. tests) + security rewiring + new
transport. The two-PR split keeps each reviewable.

**Risks:**

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| R1 | **stdio regression** — the migration touches every existing local user's path (AC #6) | High | PR 1 is stdio-only with zero intended behavior change; the ported `dispatch_test.go` wire-level golden tests are the net. Bisectable because it lands separately. |
| R2 | **Silent read leak** — a handler missed during rewiring keeps reading raw | High | Narrow `Deps` to read interfaces so raw access is *unavailable* rather than merely discouraged (the TKT-80EWGM "make the mistake impossible" pattern); AC #3 covers list, show, search AND the neighbor-title path. |
| R3 | **Coverage floor** — `internal/mcp` is ~56% against a floor of 50 (`.testcoverage.yml:41`), only ~6pp headroom; a rewrite easily dips under | Medium | Port tests in the SAME PR as the code they cover; run `just coverage-check` before pushing. |
| R4 | **Notification semantics** — assumed `resources/list_changed` would carry over; it cannot in stateless mode | Medium | Resolved in design: emit nothing remotely, document the limitation. Watcher stays stdio-only. |
| R5 | **SDK immaturity** — v1.7.0 is ~2 weeks old; 11 `MCPGODEBUG` escape hatches signal churn, all slated for removal by v1.9.0 | Medium | Do not depend on any `MCPGODEBUG` flag. Pin the version; re-review on upgrade. |
| R6 | **Origin/CSRF misconfiguration** — `/api/` is origin-gated but MCP clients send no `Origin` | Medium | Deliberate decision needed (see open question); the exemption is only sound because the JWT gate authenticates independently — document that coupling, and never exempt without an auth gate. |
| R7 | **`ErrUnstampedPrincipal` 500s** — mounting outside the middleware chain yields an unstamped principal | Low | Mount inside `registerAPIV1Routes`; AC #4/#5 would fail loudly if misplaced. |
| R8 | **Scope creep into TKT-G3PPD** | Low | Tool-list filtering stays out; this ticket only lands the seam. |

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] **`docs-project/entities/`** — the SOURCE for `docs/mcp-server.md`
  (that file is auto-generated; do not hand-edit it). New "Remote / server
  deployment" section: how to enable, the `/api/v1/_mcp` endpoint, client
  config, and the explicit statements that (a) Lua tools are stdio-only and
  (b) `resources/list_changed` is not delivered remotely.
- [x] **`docs/acl-security.md:743`** — rewrite the "MCP transport" gap entry.
  It currently says the gap is accepted *because* MCP is local-only; once the
  read gating lands that justification and the exemption both go away.
- [x] **`docs/server-security.md`** — threat model for the new network surface:
  auth posture, the Origin/CSRF decision and why it is sound, and an explicit
  "this is NOT tenant isolation" note (`principal.OrgID` is attribution only).
- [x] **`README.md` / `docs/getting-started.md`** — mention remote MCP exists.
- [x] ~~`docs/metamodel.md`~~ (N/A: no metamodel change)
- [x] ~~`docs/data-entry.md`~~ (N/A: no UI change)
- [x] **`CLAUDE.md`** — a short rule capturing the new invariant: MCP read
  paths go through injected visibility-wrapped readers; `internal/mcp` must not
  import `acl`/`visibility`; Lua stays off remote transports.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

### Findings (self-review, each verified against the code)

**D1 — `resources.go` is a SECOND ungated read surface, entirely outside the
tools. (critical)**
`rela://entity/{type}/{id}` → `handleReadEntity` reads
`s.deps.Store.GetEntity` raw (`resources.go:89`), and
`rela://relation/{from}/{type}/{to}` → `handleReadRelation` reads
`GetRelation` raw (`resources.go:124`). Gating only the *tool* handlers would
leave a fully-parallel path to the same data. **Worse:** `resources.go:94`
returns `"entity %s is type %s, not %s"` — disclosing an entity's *real type* to
a caller who may not read it at all. That is an existence-AND-type oracle, the
exact thing the row-level rule forbids.
*Resolution:* resources go through the same gated reader as tools; the
type-mismatch error must collapse into the same indistinguishable not-found.

**D2 — `prompts.go` is a THIRD ungated surface. (critical)**
7 raw sites (`prompts.go:68,80,130,132,210,238,287`). The prompt bodies embed
entity data and orphan lists directly into text handed to an LLM. Gating tools
and resources but not prompts leaves the leak intact in the least obvious place.
*Resolution:* same injected gated reader/tracer.

**D3 — SUPERSEDED. See D13 below: the analyze gating is already solved in-tree
and should be reused, not worked around.**

**D3 (original text, kept for the record) — the `analyze_*` tools aggregate
over the WHOLE graph. (significant)**
`analyze_unique` (`tools_analysis.go:171`) reports same-type entities sharing a
property value — leaking both ids **and property values**. `analyze_cardinality`
(`:108`), `analyze_properties` (`:216`, `:239`) and `analyze_schema` (`:337`)
similarly enumerate or count everything. Note `analyze_orphans` is already
covered *if* the tracer is swapped, because `visibility.VisibleTracer`
implements `FindOrphans` (`internal/visibility/tracer.go:192`), as it does
`FindPath` (`:141`) and `HasCycle` (`:221`).
*Complication:* `schema.ValidateRelationProperties(ctx, st store.Store, ...)`
(`internal/schema/validate_properties.go:54`) and `schema.StoreCounter{Store:
store.Store}` (`internal/schema/store_adapter.go:10`) take the **wide**
`store.Store`, so they cannot accept a narrowed read interface without change.
*Resolution options (decide in implementation):* (a) exclude the whole-graph
analyzers from the remote tool set — cheapest, consistent with excluding Lua;
(b) adapt them to a gated reader. **Recommend (a) for this ticket**, since
counts over a filtered graph are a distinct feature, and `filtered_count`
semantics (DEC-RG878) deserve their own design.

**D4 — the RFC 9728 challenge conflicts with the existing JWT gate.
(significant)**
`jwtgate.go:228` sets `WWW-Authenticate: Bearer` **only when the configured
header is literally `Authorization`** — but the default is `X-Auth-Assertion`
(`cmd/rela-server/main.go:108`). MCP clients send `Authorization`, and RFC 9728
requires the challenge carry `resource_metadata=…`. So AC #9 is NOT satisfiable
by adding a well-known endpoint alone: the gate's 401 path must be extended to
emit the parameterised challenge for the MCP route.
*Resolution:* extend the gate (or wrap the MCP handler with its own 401
writer); add an explicit test that the challenge appears with
`resource_metadata`.

**D5 — read-count in this plan was wrong. (minor, corrected)**
Originally "~25 `deps.Store` uses"; the true figure is **42** sites across 8
files. Corrected in Security Considerations above. The lesson is the same one
D1/D2 teach: enumerate by *surface* (tools, resources, prompts, export), not by
grepping the files you happen to be thinking about.

**Net effect on the plan:** the "inject a wrapped reader and handlers are fixed"
story still holds, but it must be applied at **four** surfaces, not one — and
the whole-graph analyzers are better excluded than gated in this ticket.
AC #3 is extended to cover resources and prompts; AC #7 gains the analyzers.

**D6 — Origin/CSRF question RESOLVED (was an open question). (minor)**
`/api/v1/_mcp` should join `nonBrowserExemptPrefixes`
(`middleware_security.go:246`) alongside `/api/sync/` and `/api/v1/_feeds/`,
conditioned as they are on `isCSRFExempt` (`:288`) — which exempts only requests
that are *provably* non-browser (no `Sec-Fetch-*`, no Cookie, no Origin/Referer).
This is not a workaround: the comment at `:239-244` states the exemption becomes
*structurally* sound when "rela validates a signed proxy assertion / Bearer
token itself and REQUIRES it here, rejecting cookie-only requests (CSRF-immune
by construction)". A JWT-required MCP endpoint is precisely that case. A browser
`fetch()` of `/api/v1/_mcp` still carries `Sec-Fetch-Site` and so still hits the
same-origin check.
*Caveat to enforce:* the exemption is only sound **while the JWT gate is
active**. If MCP is enabled without JWT identity (header-identity mode, which
fails open to `unknown`), the endpoint must NOT be exempt — or better, enabling
remote MCP without verified identity should be refused at startup, matching the
existing `validateIdentityFlags` fail-loud posture.

### Second-pass findings (adversarial review) — these changed the plan's shape

**D7 — the ctx-wrapper thesis has a hole (RR-OMB6ID, critical).**
`schema.StoreCounter` hardcodes `context.Background()`
(`internal/schema/store_adapter.go:15`,`:20`). A ctx-resolving visibility
wrapper is **inert** there — it compiles, it "has a gate", and it gates nothing.
This invalidates the mitigation stated for R2, the plan's own top risk, and
pulls an `internal/schema` refactor onto the critical path.
*Systemic fix worth doing:* a CI check for `context.Background()` in packages
reachable from a gated read path. This bug class is invisible to review and has
already occurred once.

**D8 — `export` and `analyze_validations` (RR-H7DFZ5, critical).**
`export` with no type is a full graph dump (`tools_export.go:34`).
`analyze_validations` (`tools_analysis.go:304`) has an ungated candidate set AND
**executes operator-authored Lua on the read path** — so AC #7's test (no
`lua_*` in `tools/list`) passes while Lua still runs remotely. `Deps.Validator`
was missing from the inventory entirely.

**D9 — `trace_*`/`find_path` raw pre-flight probe (RR-FTJUUE, critical).**
`tools_trace.go:42` calls `deps.Store.GetEntity` **before** the tracer, so
swapping in `VisibleTracer` does not close the oracle. Directly contradicts this
plan's own edge-case claim that "the gate runs first" — it does not.

**D10 — `Tool` stamp (RR-H8S10M, significant).**
`router.go:577` hardcodes `principal.ToolDataEntry`, so AC #4 fails; and the
naive fix (a `Principal` literal) silently drops roles because `Verified` is the
only constructor that populates them. Needs a `Verified`-preserving `WithTool`
or a threaded parameter. Adds `internal/principal` + `router.go` to the file list.

**D11 — `acl.Request` concurrency (RR-PQ5UN1, significant).**
One Request per *HTTP request* is not one per *logical operation*: a JSON-RPC
batch may dispatch handlers concurrently over the shared, unsynchronized
`globalsLoaded`. The planned `-race`-with-parallel-calls test **cannot fail**,
since parallel HTTP requests each get their own Request.

**D12 — AC #6 is not literally achievable (adopt the reviewer's restatement).**
"Byte-identical" is falsified by the migration itself: error results move to the
go-sdk convention (`tool.go:48`) and input schemas become *reflected* from
struct tags, so `tools/list` JSON will differ in ordering/`required`/
`additionalProperties`. **Restate AC #6 as "semantically equivalent, with a
documented diff for tool-schema JSON and the error-result envelope", and freeze
golden `tools/list` + representative `tools/call` outputs from the CURRENT
binary BEFORE PR 1 starts** — otherwise the ported test co-evolves with the code
it is supposed to guard.

### REVISED plan: three PRs, security before network

The reviewer is right that the original two-PR split put every security decision
plus a new network surface in one enormous PR — the exact conditions under which
D1/D2/D8 slip through. Revised:

- **PR 1 — SDK migration, stdio only.** Unchanged, but gated on the frozen
  golden artifacts (D12).
- **PR 2 — read gating, still stdio-only, still `NopACL`.** Narrow `Deps`,
  refactor `internal/schema` off concrete `store.Store` (D7), wire gated
  reader/tracer/validator/searcher, gate **all four** surfaces (tools,
  resources, prompts, export), fix the trace pre-flight (D9) and the neighbor
  titles. Behavior is unchanged under `NopACL`, so this is reviewable purely as
  "did the plumbing stay correct" — and the raw-access mistake becomes
  *unavailable* before any network surface exists.
- **PR 3 — HTTP mount, per-request principal, RFC 9728.** Now small.

**Adopt the allowlist inversion (reviewer L3).** Remote tool registration takes
an **explicit allowlist** of vetted-and-gated tools rather than a denylist of
excluded ones. A denylist means every future tool is remote-exposed by default,
which is backwards from DEC-RG878's "default-deny" — and D8 shows the risky set
was already larger than the plan's denylist. This is also the seam TKT-G3PPD
needs.

**Design Review Findings:** RR-CFFL52 (critical), RR-NSUN49 (critical),
RR-B7ZHYO (significant), RR-P34E8J (significant), RR-OMB6ID (critical),
RR-H7DFZ5 (critical), RR-FTJUUE (critical), RR-H8S10M (significant),
RR-PQ5UN1 (significant). D5, D6 and D12 corrected inline.

### D13 — the analyze gating is ALREADY SOLVED in-tree; reuse it (corrects D3)

Raised by the ticket owner, and verified: the browser's `_analyze` endpoint is
already principal-gated, so "whole-graph analysis" is a solved problem here, not
a hard one.

`analyzeService` (`internal/dataentry/analyze.go:53`) holds:

```go
type analyzeService struct {
    reads     analyzeReader     // GATED per requesting principal (TKT-3FL2S6)
    relCounts relationCounter   // raw; structural counts, cannot leak
    tracer    tracer.Tracer     // the gated decorator
    validator validator.Validator
}
```

Every check re-loads each candidate through the gated `reads` before it can
become an issue — `analyzeOrphans` (`:150-166`) carries an explicit comment
warning *not* to emit an issue straight from an orphan id, because that would
reopen the leak TKT-3FL2S6 closed. `handleV1Analyze` (`api_v1.go:1412-1419`)
then needs **no post-hoc output filter**: the issues already are the requester's
visible slice. Pinned by `TestACLAnalyze_*` in `acl_analyze_test.go`, including
the readable-but-redacted-primary-title case (BUG-R9EHKV) and an assertion that
aggregate **counts** agree with the visible issue list.

**So: gate the analyzers, don't exclude them.** MCP reuses this composition
rather than inventing one. Under `NopACL` the gated handles are the raw ones, so
stdio behavior is unchanged.

**How `dataentry` sidesteps the ctx problem (D7/RR-OMB6ID), checked:** it does
**not** use `schema.StoreCounter` at all. It declares its own narrow,
ctx-taking consumer-side interfaces — `analyzeReader` (`GetEntity` +
`ListEntities`) and `relationCounter` (`CountRelations`), `analyze.go:32-41` —
and notes that `relCounts` is raw on purpose because a structural relation count
cannot leak. That is the CLAUDE.md consumer-side-interface rule doing exactly
its job, and it is the shape MCP should copy.

So `schema.StoreCounter` has only two callers, both local/trusted surfaces:
`internal/mcp/tools_analysis.go:339` and `internal/cli/analyze.go:670`. The
`context.Background()` defect is therefore confined to **`analyze_schema`**,
which reports metamodel *type usage* (counts per declared type) rather than
reading entity rows. Options for PR 2, cheapest first: leave `analyze_schema`
ungated and say so (type names are config, not secret — CLAUDE.md); or thread
ctx through `TypeCounter`. **No broad `internal/schema` refactor is on the
critical path** — that part of D7 was overstated.

### D14 — Lua on the read path is fine (corrects the AC #7 tightening)

Also from the ticket owner: the CLAUDE.md rule "don't run user-supplied Lua on
the read path" targets **unbounded per-record work on hot list views**, not a
bounded, operator-authored validation run the caller explicitly requested. So
`analyze_validations` executing `lua_file:` rules is acceptable, and AC #7 goes
back to its original meaning — no `lua_eval`/`lua_run`/`lua_list` remotely.

The genuine half of RR-H7DFZ5 stands: the validator's **candidate set** is
ungated (`mcp_wiring.go` passes `svc.Validator()` over the raw store), and
`export` dumps the whole graph unfiltered. Both are fixed by the same injected
gated reader.

**Status: PR 1 COMPLETE** (see the ticket). PR 2 is now better-scoped than at
first review: the pattern to copy exists, so the work is wiring MCP's `Deps` to
gated handles across all surfaces (tools, resources, prompts, export, analyze,
validator) plus the trace pre-flight fix (D9/RR-FTJUUE), not inventing gating.
Re-estimate downward from the original XL.

**Design Review Findings:** <!-- List review-response IDs, e.g., RR-xxxx -->
