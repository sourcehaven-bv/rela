<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# Security model for rela-server

`rela-server` is the HTTP data-entry app shipped with rela. It is intended to
run on a local port (`http://localhost:8080` by default) and be opened in your
normal browser.

This guide documents the threat model the server is hardened against, the
defenses it employs, the residual risks, and the configuration knobs available
to operators and developers. For the ACL system itself — the resolver
vocabulary, role configuration, and the read-side filtering guarantees — read
[GUIDE-acl-overview] and [GUIDE-acl-security]; this guide summarizes how that
coverage stands and links out rather than restating it.

## Threat model

The server runs on your machine, but your machine is not a closed system: any
website you visit can execute arbitrary JavaScript in a browser tab, and that
JavaScript can issue HTTP requests to `http://localhost:<port>` just like the
data-entry SPA does. Without active defenses, a malicious page could:

- Read live project events from the SSE endpoint (file changes, entity
  updates, git status).
- Create / update / delete entities in your project via cross-origin
  `fetch` requests.
- Trigger any configured shell command via `/api/command/`.
- Open arbitrary local files via `/api/open-file`.
- Pivot via DNS rebinding to bypass loopback assumptions.

The threat model assumes:

- The attacker is a website loaded in your browser, executing JavaScript.
- The attacker can also use simple HTML primitives (`<img src=…>`,
  `<form>` POST) that bypass JavaScript-only defenses.
- The attacker controls DNS for hostnames they own (DNS rebinding).
- The attacker does **not** have local code execution on your machine.
  Local malware running with user privileges is out of scope; any program
  running as the current user can already do everything `rela-server` can.

## Defenses

`rela-server` enforces the following on every HTTP request:

### 1. Loopback binding by default

The server binds to `127.0.0.1` by default. Other machines on your LAN
cannot reach the server unless you explicitly opt in with `--bind`.

```sh
# Default: only this machine can reach it
rela-server

# Opt in to LAN access (review threat model first!)
rela-server --bind 0.0.0.0 --allowed-origin http://my-laptop.local:8080
```

When the bind address is non-loopback, the server prints a prominent warning
at startup. **You must also pass `--allowed-origin` for every hostname your
clients will use to reach the server**, otherwise their requests will be
rejected by the Origin allowlist (see §3 below). Common examples:

- `--allowed-origin http://192.168.1.5:8080` (LAN IP)
- `--allowed-origin http://my-laptop.local:8080` (mDNS / Bonjour)
- `--allowed-origin https://rela.example.com` (behind a reverse proxy with TLS)

When bound to `0.0.0.0` or `::` the **Host header check is disabled** (we
cannot enumerate the legitimate Host values ahead of time). The Origin
allowlist becomes the only CSRF gate in that mode, so make sure your
`--allowed-origin` set is accurate.

### 2. Host header allowlist (DNS rebinding defense)

Every request must carry a `Host` header matching the bound address (or one
of the loopback aliases when bound to loopback). Requests with spoofed Host
headers — the hallmark of a DNS rebinding attack — are rejected with `403`.

### 3. Origin allowlist on sensitive endpoints

Every request to `/api/...` must carry an `Origin` header (or `Referer`
fallback) matching the server's own origin. Requests from other origins are
rejected with `403`.

The check applies on **every** HTTP method, including `GET`. This is
important: some endpoints (notably `/api/command/...`) are state-changing
even on `GET`, and a method-based filter would let `<img src=...>` style
attacks through.

Static assets (`/static/`, the SPA shell) are exempt — they leak no project
data and need to remain fetchable cross-origin in some setups.

### 4. SSE endpoints are same-origin only

`/api/events` and `/api/v1/_events` no longer reflect the request `Origin`
back as `Access-Control-Allow-Origin` (which previously let any website
subscribe to your live project events). They are protected by the same
Origin allowlist as the rest of `/api`.

### 5. Path containment in `/api/open-file`

The `path` parameter is cleaned, made absolute, and resolved through any
symlinks. Requests that resolve to a location outside the project root are
rejected with `403`. Paths with NUL bytes are also rejected.

### 6. URL scheme allowlist in `/api/open-url`

Only `http`, `https`, and `mailto` URLs are accepted. `file://`,
`javascript:`, `data:`, and other potentially dangerous schemes are
rejected.

### 7. Per-request timeouts

`http.Server.ReadHeaderTimeout`, `ReadTimeout`, and `IdleTimeout` are set
to bound resource use. `WriteTimeout` is intentionally `0` (unlimited):
Server-Sent Events and command-exec output stream long-lived responses,
and a write deadline would kill them mid-flight. Slow-write protection
is provided by `IdleTimeout` and (in the future) by per-handler context
deadlines on individual mutating handlers.

## Audit logging

Every entity / relation create / update / delete is recorded as a
JSONL row under `.rela/audit/YYYY-MM-DD.jsonl`. Records carry the
operating user (`$USER`), the entry point that initiated the write
(`cli`, `mcp`, `data-entry`, `scheduler`, `desktop`), and — for
engine-initiated writes — the originating automation or schedule.

The log is forensic, not authoritative: a process crash between the
store write and the audit append can leave a write un-audited; see
[GUIDE-audit-log] for the durability story, the JSONL schema, and `jq`
recipes for common queries.

`.rela/audit/` is gitignored by convention — audit content is
per-machine and should not be committed.

### Retention

`rela` **never deletes audit logs**. The backend rotates to a new
`YYYY-MM-DD.jsonl` file each UTC day and appends; it has no pruning,
expiry, or cleanup path. The default behaviour is therefore
retention-safe — the application cannot, on its own, drop a record
below any required retention window.

Retention is an **operational control**, owned by the deployment, not
the application. For environments subject to a security-log retention
requirement (e.g. **POLICY-017 §4 / PROCEDURE-f4cu: security logs
retained ≥ 12 months**):

- Retain everything under `.rela/audit/` for **at least 12 months**.
  Back it up or ship it off-box if the working tree is ephemeral
  (containers, CI runners, re-provisioned hosts), since the directory
  is gitignored and lives only on the local disk.
- If you prune at all, prune **only beyond** the retention window. The
  daily file naming makes this exact: delete files whose date is older
  than your window, never on a shorter `-mtime`. See [GUIDE-audit-log]
  for a compliant example.

There is no built-in enforcement of the minimum — `rela` cannot police
an operator's `rm`. The guarantee it provides is the converse: it will
not delete logs for you.

### `data-entry` user attribution

By default the data-entry server records `principal.user: "unknown"`
on every audit row — the server-process `$USER` would be misleading
for human web users. Two opt-in sources can replace the placeholder:

- **`--principal-header X-Forwarded-User`** (or any header name) on
  `rela-server`. The middleware reads the named header on every
  request and stamps its value as `principal.user`.
- **`$RELA_DATAENTRY_USER`** env var, set on the `rela-server`
  process. Useful for local development where there's no proxy.
  The env value wins over the header.

**Trust boundary**: the `--principal-header` flag is only safe
behind a reverse proxy that

1. **strips** the same header from inbound requests, and
2. **sets** it from an authenticated source (oauth2-proxy, Vouch,
   traefik forward-auth, etc.).

A direct-to-data-entry deployment must not enable this flag —
clients can spoof the header at will. If `rela-server` is bound
beyond loopback (`--bind` non-loopback) and `--principal-header`
is set, audit attribution is only as trustworthy as the network
path between the client and the proxy.

Header values are sanitized at the middleware (trim, 256-rune cap,
control-char strip) as defense-in-depth against header-injection
corrupting the JSONL stream.

## Access control (`acl.yaml`)

rela-server enforces a declarative ACL at every write entry point, and — when
a policy is configured — filters reads. The policy lives at `acl.yaml` at the
project root (alongside `metamodel.yaml`). Three modes:

| Mode | How | Behavior |
|---|---|---|
| **Open** (default) | No `acl.yaml` present | Every authenticated request can write. Reads have no filtering. Suitable for single-user local projects. |
| **Read-only** | `rela-server --read-only` or `RELA_READ_ONLY=1` | Every write returns HTTP 403; reads unaffected. Useful for demos, maintenance, observe-only deployments. Wins over `acl.yaml` — explicit flag overrides policy. |
| **Policy** | `acl.yaml` present | Writes are gated by role assignments and delegate permissions. Reads are filtered on the data-entry HTTP surface: per-entity GETs 404 like not-found for hidden entities; lists / sidebar counts / pagination / `?include=` / `/_position` / `/_search` return only the visible subset; and `visible:`-denied properties are redacted from every response body. MCP read surfaces are not yet filtered. See [GUIDE-acl-security]. |

A startup warning fires when the server binds **beyond loopback**
(`--bind` non-loopback) **without** `acl.yaml` AND **without**
`--read-only` — that combination means anyone reachable on the
network can write to the project.

**Configuring the policy** — roles, per-verb grants, delegate-X tamper
resistance, group membership (`member-of`) hardening, the trust boundary on
`principal.user`, and the field-/option-/relation-level affordances driven by
`fields:` / `visible:` / `options:` / `relations:` grants — is covered in
[GUIDE-acl-overview] (the model) and [GUIDE-acl-security] (operator
hardening). The rest of this section documents only what the ACL **covers as a
defense** in the server threat model.

### What the ACL covers

- ✅ Write authz at every `Manager.{Create,Update,Delete}{Entity,Relation}` + `RenameEntity`.
- ✅ HTTP 403 with structured `{error, rule_kind, rule_id, reason}` body.
- ✅ Audit log records every deny as `denied-write` (see [GUIDE-audit-log]).
- ✅ Data-entry SPA hides entity-CRUD write controls based on the
  per-resource `_actions` verdict map — read-only mode produces a
  UI with no "+ New", delete, or Edit buttons for entities,
  driven by the ACL with no frontend flag. Deferred phase-2 sites
  (Lua command buttons, settings / theme / git writes, relation
  add/remove inside form widgets) remain visible and 403 at the
  server on click; later phases gate them as new verbs land.
- ✅ Field-, option-, and relation-level affordances driven by
  per-role `fields:` / `visible:` / `options:` / `relations:` grants
  with optional `when:` predicates. See [GUIDE-acl-security].
- ✅ **Entity-level read filtering** on the data-entry HTTP surface.
  Per-entity GETs 404 like not-found for hidden entities; lists,
  sidebar counts, pagination, `?include=` neighbours, `/_position`,
  and `/_search` return only the visible subset. See [GUIDE-acl-security].
- ✅ **Property-level redaction** (`visible:` grants) on every
  data-entry HTTP read. A field denied by `visible:` is omitted from
  the response `properties` map on per-entity GET, list rows,
  `?include=` peers, and `/_search` results — not just the write
  form. When the hidden field is the display property, the title
  falls back to the entity ID so the redacted value can't leak
  through `_title`.
- ✅ Group expansion (`member-of`, transitive) and inherited local
  roles (containment, via `inherit_roles_through`). Direct local
  roles are honored as well.
- ❌ **MCP read surfaces are not yet ACL-filtered.** `show_entity`,
  `list_entities`, `search_entities`, and the trace tools return
  full entity bodies with no entity-level gate and no `visible:`
  redaction. The MCP server is local-only (stdio), so this is an
  accepted gap at this stage, tracked as a follow-up. Do not expose
  the MCP transport to an untrusted caller while relying on `visible:`
  for confidentiality.
- ❌ **Search match-on-hidden-field oracle.** `/_search` redacts the
  *body* of a hidden property but the search index still matches on
  its text, so a hit can confirm a hidden value's presence by
  appearing in results (e.g. searching a candidate postcode). Closing
  this — dropping hits that matched only on a hidden field — is a
  tracked follow-up. Treat `visible:` as hiding values from view, not
  as making them unguessable via search.
- ❌ MCP transport intersection (filtering the tool list per principal) — deferred to a follow-up.

> **`_actions` is a UI hint, not an authorization layer.** The
> data-entry server re-authorizes every write — a client that
> bypasses the affordance map (forges `delete: true` and issues
> DELETE anyway) gets the same `403` the policy would have produced
> for any other denied request. The scope of the affordance
> invariant is HTTP write endpoints reached by the SPA; MCP / Lua /
> scheduler write paths share the same enforcement but do not emit
> or consult `_actions`.

## Running the Vue dev server (Vite)

If you run the SPA via Vite on `http://localhost:5173`, requests to the Go
backend will carry `Origin: http://localhost:5173`, which is **not** in the
default allowlist. Tell `rela-server` to permit that origin:

```sh
rela-server --allowed-origin http://localhost:5173
```

The flag is repeatable. Each value must be a complete origin
(`scheme://host:port`).

## Calling the API from curl, scripts, or non-browser clients

The Origin allowlist treats requests with no `Origin` and no `Referer` header
as cross-origin and rejects them with `403 forbidden` and reason
`origin_missing`. This catches `<img src=...>` style attacks where the
attacker has set `Referrer-Policy: no-referrer` to strip both headers.

It also rejects bare `curl http://localhost:8080/api/...` calls. To use the
API from the command line, set the Origin header explicitly:

```sh
curl -H 'Origin: http://localhost:8080' http://localhost:8080/api/v1/_config
```

The same applies to any script, MCP integration, or test harness that speaks
HTTP directly to `rela-server`.

## Troubleshooting

**"403 forbidden" with reason `host_not_allowed`** — your client sent a
`Host` header that doesn't match the bound address. If you're hitting the
server from another machine, either rebind to that interface (`--bind ...`)
or check whether DNS rebinding is in play.

**"403 forbidden" with reason `origin_not_allowed`** — your client sent an
`Origin` header that isn't in the allowlist. Add it via `--allowed-origin`
or run from a same-origin context.

**"403 forbidden" with reason `origin_missing`** — neither `Origin` nor
`Referer` was present. See "Calling the API from curl" above.

**SSE / live reload not working in Vite dev mode** — check that the Vite
proxy in `frontend/vite.config.ts` forwards `/api/events` and that you
passed `--allowed-origin http://localhost:5173`.

## Residual risks and known limitations

The following risks are **not** fully mitigated by the defenses above. They
are documented here so operators can make informed decisions.

### TOCTOU window in `/api/open-file`

There is a small time-of-check / time-of-use window between the path
containment check and the synchronous invocation of the OS open command
(macOS `open`, Linux `xdg-open`, Windows `explorer`). An attacker with
local filesystem write access could swap a contained path for a symlink
during that window.

This is an accepted residual because:

- The local filesystem is the trust boundary (anything that can write
  files in your project can already cause harm directly).
- Portable mitigation (file-descriptor passing through `open`/`xdg-open`/
  `explorer`) does not exist.

### No authentication

There is intentionally no login or per-user authentication. The trust
boundary is "anything running as the current user on this machine."
Per-instance session tokens (defense in depth on top of the Origin
allowlist) are tracked as a follow-up.

### Configured commands are remote-code-execution by design

The `commands` section of `data-entry.yaml` lets you wire up arbitrary
shell scripts that run with your user privileges. Be careful what you put
there. The `/api/command/` endpoint is `POST`-only and protected by the
Origin allowlist, but the scripts themselves are still trusted code.

### Future WebSocket endpoints need explicit Origin checks

WebSockets are not currently used by `rela-server`. If a future feature
adds them, note that the browser does **not** enforce same-origin policy
on WebSocket upgrades — the upgrade handler must explicitly check
`Origin` itself, the same way the existing `requireSameOrigin` middleware
does for HTTP requests.

## Reporting vulnerabilities

If you discover a security issue not covered here, please open an issue
on the GitHub repository or contact the maintainers privately.
