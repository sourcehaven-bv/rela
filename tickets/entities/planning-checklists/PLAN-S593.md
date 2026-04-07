---
id: PLAN-S593
type: planning-checklist
title: 'Planning: Harden rela-server against browser-based local attacks'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** `rela-server` is intended to be run permanently on a local port,
but its current implementation assumes "anything that can talk to localhost is
trusted." That assumption is wrong: a web browser visiting any other site is
already inside the trust boundary. A malicious page can use cross-origin
`fetch`/`EventSource`/`<img>` to read project state, mutate entities, open
arbitrary files, and trigger configured shell commands. DNS rebinding lets the
same page keep talking to the loopback server even with a spoofed `Host` header.

**Scope:**

IN scope:
- HTTP layer hardening of `cmd/rela-server` and `internal/dataentry` handlers.
- Loopback-only binding by default with explicit `--bind` opt-in.
- `--allowed-origin` flag (repeatable) for dev workflows (Vite on :5173).
- Origin/Host allowlist on every sensitive endpoint (mutating *and* sensitive
read endpoints — not just non-safe HTTP methods).
- Removing CORS reflection from `/api/events`.
- Path containment in `/api/open-file`.
- Scheme allowlist in `/api/open-url` (newly identified scope).
- Restricting `handleCommandExec` to POST only (newly identified scope).
- Allowlist validation of relation type segment in API v1 URL parsing
(reclassified Critical after design review).
- Defensive sanitisation of `WriteCacheFile` and `RelationFilePath`.
- Per-handler context deadlines on mutating handlers; full timeouts on
`http.Server` *except* streaming routes.
- Integration tests for each rejection path.
- `docs/security.md` documenting threat model, dev setup, residual risks.

OUT of scope:
- Authentication / multi-user accounts.
- Per-instance random session token (defence-in-depth, follow-up ticket).
- Wails desktop app IPC review (separate ticket if needed).
- Lua sandbox hardening (already covered by `lua-sandbox-tests`).
- MCP server (separate process, stdio transport, not network-exposed).
- WebSocket support (none today; documented as a future hardening requirement).

**Acceptance Criteria:** (each maps to a test scenario in Test Plan below)

1. Default startup binds to `127.0.0.1`. `--bind 0.0.0.0` works as escape hatch.
2. POST/PUT/PATCH/DELETE to any sensitive endpoint with
`Origin: https://evil.example` → 403.
3. **GET** to `/api/command/{id}`, `/api/open-file`, `/api/open-url` with bad
Origin → 403 (sensitive endpoints check Origin on **all** methods).
4. `handleCommandExec` rejects GET with `405 Method Not Allowed`.
5. GET `/api/events` with `Origin: https://evil.example` → 403; response has
no `Access-Control-Allow-*` headers.
6. Any request with `Host: evil.example` → 403.
7. `/api/open-file?path=/etc/passwd` → 403; `path=../../etc/passwd` → 403;
`path=entities/foo.md` → 200; symlink-out-of-project → 403; NUL byte → 403.
8. `/api/open-url?url=file:///etc/passwd` → 403; `url=javascript:...` → 403;
`url=https://example.com` → 200.
9. `POST /api/v1/tickets/TKT-1/relations/..%2Fevil/TKT-2` → 400, file is **not**
written.
10. `WriteCacheFile("../escape.yaml", ...)` returns error.
11. `cmd/rela-server/main.go` `http.Server` has all four timeouts set; SSE and
`/api/command/*` handlers stream successfully for >2 s.
12. Vue dev server (Vite on :5173) keeps working with
`--allowed-origin http://localhost:5173`.
13. Blocked requests return `{"error":"forbidden","reason":"<rule>"}` and emit
a single warn-level log line.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- **Origin/CSRF**: Go stdlib `r.Header.Get("Origin")`/`Referer` is sufficient
for an allowlist check. `gorilla/csrf` is overkill (cookie-based, requires SPA
token plumbing). Origin allowlist is the industry-standard approach for
local-only servers (Jupyter, ollama, Syncthing).
- **DNS rebinding**: same approach (Host header allowlist) used by Jupyter,
vscode-server, Plex, ollama.
- **Path containment**: idiomatic Go pattern `filepath.Clean` + `Abs` +
`EvalSymlinks` + `HasPrefix(root+sep)`.
- **Codebase patterns**: router (`internal/dataentry/router.go`) is plain
`net/http.ServeMux` (NOT chi). Existing `reloadLockMiddleware` shows how to wrap
a handler. Security middlewares slot at the top level wrapping the whole
returned handler.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Endpoint inventory (verified against the codebase)

| Method | Path | Handler | Sensitivity |
|---|---|---|---|
| POST | `/api/toggle-checkbox` | handleToggleCheckbox | mutating |
| GET/POST | `/api/command/{commandID}` | handleCommandExec | **RCE — restrict to POST + Origin** |
| POST | `/api/command-cancel/{execID}` | handleCommandCancel | mutating |
| POST | `/api/open-file` | handleOpenFile | file disclosure + Origin |
| POST | `/api/open-url` | handleOpenURL | scheme allowlist + Origin |
| POST | `/api/git/sync` | handleGitSync | mutating |
| GET | `/api/events`, `/api/v1/_events` | handleSSE | sensitive read (Origin, no CORS reflection) |
| POST | `/api/v1/_settings` | handleAPISettingsCRUD | mutating |
| POST | `/api/v1/_palette` | handleAPIPaletteCRUD | mutating |
| POST | `/api/v1/_conflicts/` | handleV1ConflictResolve | mutating |
| POST | `/api/v1/{plural}/` | handleV1CreateEntity | mutating |
| PATCH | `/api/v1/{plural}/{id}` | handleV1UpdateEntity | mutating |
| DELETE | `/api/v1/{plural}/{id}` | handleV1DeleteEntity | mutating |
| POST/PATCH/DELETE | `/api/v1/{plural}/{id}/relations/{relType}[/{targetId}]` | handleV1*Relation | mutating + relType allowlist |
| POST | `/api/entities` | handleAPICreateEntity | mutating |
| PUT/PATCH | `/api/entities/{id}` | handleAPIUpdateEntity | mutating |
| DELETE | `/api/entities/{id}` | handleAPIDeleteEntity | mutating |
| POST | `/api/relations` | handleAPICreateRelation | mutating |
| DELETE | `/api/relations` | handleAPIDeleteRelation | mutating |

Static asset routes (`/static/`, `/v2/`, `/`) get the lighter Host-only check.

### Technical Approach

1. **Loopback binding + allowed-origin flag (Critical #1, dev workflow)**
   - In `cmd/rela-server/main.go`:
     - Add `--bind` (string, default `"127.0.0.1"`).
     - Add `--allowed-origin` (repeatable string slice, default empty).
     - Compose `Addr = *bind + ":" + *port`.
     - Log the actual bind address; warn loudly when bind is non-loopback.
   - Pass bind + allowed-origin into the dataentry app config so middlewares
can build their allowlists.

2. **`requireLocalHost` middleware (High — DNS rebinding)**
   - New file `internal/dataentry/middleware_security.go`.
   - Allowlist built from configured bind address + loopback aliases
(`127.0.0.1:<port>`, `localhost:<port>`, `[::1]:<port>`).
   - Empty Host → reject. Mismatch → reject 403.
   - Applies to **every** request (wraps the whole handler).

3. **`requireSameOrigin` middleware (Critical #2, #3, #4, #5, dev fix)**
   - Same file. Applies to all sensitive endpoints (see inventory) on **all
methods**, not just non-safe ones — `<img src=>`/`<link>` etc. are GETs.
   - Allowlist construction:
     - Always: `http://127.0.0.1:<port>`, `http://localhost:<port>`,
`http://[::1]:<port>`.
     - Plus every value passed via `--allowed-origin`.
   - Origin matching algorithm:
     - If `Origin` header missing: fall back to `Referer` (parse with
`url.Parse`, take scheme+hostname+port).
     - If both missing: reject.
     - If `Origin == "null"`: reject explicitly.
     - Parse with `url.Parse`, lowercase scheme + hostname, normalise default
ports (`http→80`, `https→443`), reject trailing slash, compare against allowlist
by exact field equality.
   - Sensitive endpoints are matched by path prefix; the list lives in one
`var sensitivePaths = []string{...}` constant for easy review.

4. **SSE CORS removal (Critical #5)**
   - In `internal/dataentry/watcher.go`, delete the
`Access-Control-Allow-Origin` and `Access-Control-Allow-Credentials` header
writes (the four lines around 219–224).
   - SSE routes are in the sensitive-paths list, so `requireSameOrigin`
handles cross-origin EventSource attempts.
   - **Do not** wrap SSE in any timeout middleware (see #8).

5. **`handleCommandExec` POST-only (Critical RR-1A1K)**
   - In `internal/dataentry/commands.go`, add an early
`if r.Method != http.MethodPost { http.Error(w, "method not allowed", 405);
return }`.
   - Update existing handler tests / SPA call sites to POST.

6. **`handleOpenFile` path containment (Critical #4 + RR-ZI1V)**
   - Resolve: `clean := filepath.Clean(filePath)`,
`abs, _ := filepath.Abs(clean)`, `resolved, _ := filepath.EvalSymlinks(abs)`.
   - `root, _ := filepath.EvalSymlinks(filepath.Clean(a.ProjectRoot()))`.
   - Reject unless `resolved == root` or
`strings.HasPrefix(resolved, root+string(os.PathSeparator))`.
   - Reject if path contains a NUL byte.
   - Document the small TOCTOU window in `docs/security.md` as accepted
residual risk.

7. **`handleOpenURL` scheme allowlist (Significant RR-FFFY)**
   - Parse with `url.Parse`. Allow only `http`, `https`, `mailto`. Reject
`file://`, `javascript:`, `data:`, etc.

8. **Server timeouts without breaking SSE (Significant RR-LJ4D)**
   - In `cmd/rela-server/main.go` set:
     - `ReadHeaderTimeout: 10*time.Second`
     - `ReadTimeout: 30*time.Second`
     - `WriteTimeout: 0` (unlimited — required for streaming)
     - `IdleTimeout: 120*time.Second`
   - In each mutating handler, derive a per-request context with
`context.WithTimeout(r.Context(), 30*time.Second)` and use it for the graph
mutation. Streaming handlers (`/api/events`, `/api/v1/_events`,
`/api/command/*`) explicitly do NOT add a deadline.
   - Document the streaming-handler list in code comments.

9. **`WriteCacheFile` filename validation (Medium #7)**
   - Validate at function entry: reject if `filepath.Base(name) != name` or
name contains `..`. Return `errors.New("invalid cache filename")`.

10. **Relation type allowlist at the API v1 router (Critical RR-7CB0)**
    - In `handleV1EntityRelationType` (and any other site that extracts
`relType` from a URL segment), check `relType` against the metamodel's list of
declared relation types **before** calling `RelationFilePath` or anything else
that touches the filesystem. Unknown → 400.
    - Defence-in-depth: also add a panic/error in `RelationFilePath` if any of
`from`, `relType`, `to` contains `/`, `\`, `..`, or NUL.

11. **Router composition (Significant RR-W5LD)**
    - In `internal/dataentry/router.go`, the returned handler is wrapped from
outside in: `requireLocalHost(requireSameOrigin(reloadLockMiddleware(mux)))`.
    - `requireSameOrigin` consults its `sensitivePaths` allowlist; non-matching
requests pass through.

12. **Error response & logging (Nit RR-ZXYX)**
    - Rejection helper writes `403` with body
`{"error":"forbidden","reason":"<rule>"}` and emits one warn-level log line
`security: blocked rule=<rule> host=<host> origin=<origin> path=<path>` (Origin
truncated to 200 chars).

**Files to modify:**

| File | Change |
|---|---|
| `cmd/rela-server/main.go` | `--bind` and `--allowed-origin` flags, set timeouts (WriteTimeout=0), warn on non-loopback bind |
| `internal/dataentry/middleware_security.go` *(new)* | `requireLocalHost`, `requireSameOrigin`, sensitivePaths constant, rejection helper |
| `internal/dataentry/router.go` | Wrap returned handler with new middlewares (outside `reloadLockMiddleware`) |
| `internal/dataentry/watcher.go` | Remove CORS reflection on `/api/events` |
| `internal/dataentry/commands.go` | POST-only `handleCommandExec`; path containment in `handleOpenFile`; scheme allowlist in `handleOpenURL` |
| `internal/dataentry/handlers_v1.go` (or wherever URL parsing lives) | Allowlist relType against metamodel before filesystem access |
| `internal/repository/repository.go` | Validate `WriteCacheFile` filename |
| `internal/project/context.go` | Defensive check in `RelationFilePath` |
| `internal/dataentry/middleware_security_test.go` *(new)* | Unit tests for both middlewares |
| `internal/dataentry/server_security_test.go` *(new)* | Integration tests across the endpoint inventory |
| `docs/security.md` *(new)* | Threat model, `--bind`/`--allowed-origin` docs, dev setup, residual risks (TOCTOU, no auth, future WS guidance) |

**Alternatives considered:**

- *CSRF tokens via cookie + header*: requires SPA plumbing and session state.
Origin allowlist is sufficient for the threat model and ships in one PR. Defer
tokens to a follow-up.
- *Authentication / login*: out of scope.
- *Auto-allow all loopback ports*: rejected — widens allowlist permanently
and hides the dev exception. Explicit `--allowed-origin` flag is visible.
- *http.TimeoutHandler on JSON routes*: rejected — buffers responses, breaks
any handler that uses Flusher (RR-LJ4D). Use per-handler `context.WithTimeout`.
- *Per-route middleware via chi*: rejected — repo uses stdlib ServeMux; adding
chi for this is over-engineering. Top-level wrap is fail-closed.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

| Source | Validation | On invalid |
|---|---|---|
| `Host` header | Allowlist of bound address + loopback aliases | 403 |
| `Origin` header (sensitive paths) | Allowlist; explicit reject `null` | 403 |
| `Referer` fallback | Parse, allowlist scheme+host+port | 403 |
| `path` query in `/api/open-file` | Clean+Abs+EvalSymlinks, must live inside project root, no NUL bytes | 403 |
| `url` query in `/api/open-url` | Scheme allowlist (http/https/mailto) | 403 |
| `relType` URL segment in API v1 | Allowlist against metamodel | 400 |
| `WriteCacheFile` filename | Must equal `filepath.Base(name)` | error |
| `--bind` flag | Parsed by stdlib; warn if not loopback | warn only |
| `--allowed-origin` flag | Parsed by `url.Parse`; rejected if invalid | startup error |

**Security-Sensitive Operations:**

- Entity create/update/delete → CSRF-protected via Origin allowlist.
- Settings/palette save → same.
- Command script execution → POST-only + Origin; documented as RCE-by-design.
- File reveal/open → path-contained + Origin.
- URL open → scheme allowlist + Origin.
- File watcher SSE → Origin-protected; no CORS reflection.
- All of the above → behind Host header allowlist (DNS rebinding defence).

Error responses use generic strings — no internal paths or stack traces.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC | Test |
|---|---|
| 1 | `TestServerBindsLoopbackByDefault` — start server, dial 127.0.0.1 OK; dial public IP fails |
| 2 | `TestMutatingRejectsCrossOrigin` — table over every mutating endpoint with bad Origin → 403 |
| 3 | `TestSensitiveGetRejectsCrossOrigin` — `/api/command/x`, `/api/open-file`, `/api/open-url` → 403 even on GET |
| 4 | `TestCommandExecPostOnly` — GET → 405 |
| 5 | `TestSSERejectsCrossOrigin` — bad Origin → 403; assert no `Access-Control-*` headers |
| 6 | `TestRejectsHostHeaderSpoof` — `Host: evil.com` → 403 |
| 7 | `TestOpenFilePathContainment` — table over `/etc/passwd`, `../../etc/passwd`, `entities/x.md`, symlink-out, NUL byte |
| 8 | `TestOpenURLSchemeAllowlist` — table over `file:`, `javascript:`, `data:`, `https:` |
| 9 | `TestRelationTypeAllowlist` — `..%2Fevil` → 400, no file written; valid type → 201 |
| 10 | `TestWriteCacheFileRejectsTraversal` — `..`, `a/b`, `a\b`, `.`, empty |
| 11 | `TestServerTimeoutsAndStreaming` — assert all four timeouts; assert SSE streams >2s; assert command exec streams |
| 12 | `TestAllowedOriginFlagAcceptsViteDev` — `--allowed-origin http://localhost:5173`, request from that origin → 200 |
| 13 | `TestRejectionResponseFormat` — assert JSON body and log line format |

**Edge Cases:**

- Empty `Host` header (HTTP/1.0) → reject.
- `Host: 127.0.0.1` (no port) → reject.
- `Host: 127.0.0.1:8080` → allow.
- IPv6 loopback `[::1]:8080` → allow.
- `Origin: null` (sandboxed iframe) → reject.
- `Origin: HTTP://LOCALHOST:8080` (case) → allow (normalised).
- `Origin: http://localhost:8080/` (trailing slash) → reject.
- `Origin: http://localhost` (no port for http) → match `:80`.
- Cross-origin preflight `OPTIONS` → handled in middleware: respond 204 only
for allowed origins; the existing `OPTIONS` handlers in API v1 are left in place
but only reachable when Origin check passes.
- SSE long-lived connection → `WriteTimeout=0` lets it stream indefinitely.
- Symlinks in `/api/open-file` → `EvalSymlinks` before containment check.
- Symlink inside project pointing outside → rejected.
- File path with NUL byte → reject.
- Concurrent SSE connections from multiple legitimate tabs → unaffected.
- HEAD request to a sensitive endpoint → treated like GET (Go stdlib auto-serves).

**Negative Tests:**

- Cross-origin POST → 403, not 200, not 500.
- Cross-origin `<img src=/api/command/x>` → 403, no command runs.
- Spoofed Host → 403.
- Path traversal → 403, file is NOT opened (assert OS command was not invoked
via fake exec).
- Cache filename with `..` → error returned, no file written.
- Relation type `..%2Fevil` → 400, no file written.
- Origin matches but Host doesn't (or vice versa) → 403 (both checks
independent).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

| Risk | Mitigation |
|---|---|
| Breaking users who run the server on a non-loopback address (Docker, reverse proxy) | `--bind` flag; document in `docs/security.md`; allowlist built from configured bind |
| Breaking the Vite dev workflow on :5173 | `--allowed-origin` flag, documented in dev setup |
| SSE/command-exec streaming breaks under timeouts | `WriteTimeout=0`; per-handler context only on mutating handlers; integration test asserts streaming |
| Vue SPA fetch loses `Origin` header in some build mode | Fall back to `Referer`; integration test the SPA against the new middleware |
| `RelationFilePath` change cascading into unrelated callers | Keep change minimal; allowlist at the API parser, defensive panic in path builder |
| TOCTOU race in `/api/open-file` | Documented as accepted residual risk; trust boundary is the local filesystem |
| False sense of security from origin allowlist alone | `docs/security.md` notes session token follow-up |
| Future WebSocket additions skipping Origin checks | `docs/security.md` calls out WS as a special case requiring explicit Origin gate |

**Effort:** L (multiple files, integration tests across the endpoint inventory,
careful streaming handling, docs).

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] User guide / reference docs — new `docs/security.md` page
- [x] CLI help text — `--bind` and `--allowed-origin` flag help and warning
- [x] ~~CLAUDE.md~~ (N/A: no new reusable cross-package patterns)
- [x] README.md — short note + link to security doc
- [x] ~~API docs~~ (N/A: no public API surface change)
- [x] ~~catch-all~~ (N/A: items above cover scope)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-1A1K (critical, addressed) — handleCommandExec accepts GET → POST-only + Origin on all methods on sensitive paths
- RR-7CB0 (critical, addressed) — RelationFilePath path traversal exploitable via API v1 → allowlist relType at parser
- RR-NM5I (significant, addressed) — Vite dev server blocked → `--allowed-origin` flag
- RR-LJ4D (significant, addressed) — `http.TimeoutHandler` breaks SSE → per-handler context.WithTimeout, WriteTimeout=0
- RR-W5LD (significant, addressed) — Router is stdlib ServeMux → top-level wrap composition
- RR-FFFY (significant, addressed) — Endpoint inventory missed `handleOpenURL`/`handleGitSync` → full inventory + scheme allowlist
- RR-ZI1V (minor, addressed) — `/api/open-file` Clean+Abs+EvalSymlinks order + boundary `==` check + TOCTOU note
- RR-I1CZ (minor, addressed) — Origin matching edge cases (`null`, case, trailing slash, default ports)
- RR-ZXYX (nit, addressed) — Consistent rejection JSON + log line format
- RR-1F60 (nit, addressed) — Future WebSocket guidance in docs/security.md
