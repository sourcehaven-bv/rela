---
id: TKT-T4H4YK
type: ticket
title: 'Sync 5/5: rela CLI sync client — index, topo-ordered diff, push/pull, manual --force'
kind: enhancement
priority: medium
effort: m
status: done
---

Sub-ticket of TKT-WE01O5 / FEAT-NJ9FEN. Addresses design-review RR-YHGJHG
(crit). Depends on sync 1 (hash) + sync 4 (server API). Local apply uses
ApplyEntity (sync 3).

## Scope

- **Sync index** `.rela/sync-state.json`: `{ records: { id -> content-hash },
cursor: <opaque blob> }`. Local dirty-detection = recompute hash (sync 1) of
each working record, compare to index.
- **`rela sync push`**: for each locally-dirty record, `PUT /api/sync/<kind>/<id>`
with `If-Match: <index-hash>`. On 200 update index; on 412 record a conflict and
HALT that record with a clear report; on 422 surface the validation error.
- **`rela sync pull`**: `GET /api/sync/manifest?cursor=`. Diff each entry vs index:
hash differs + local clean → fetch content, apply locally via local
entitymanager `ApplyEntity` (automation-suppressed); `null` → mirror delete;
hash == index → no-op; hash differs + local ALSO dirty → conflict, halt+report.
Advance cursor only past confirmed-applied records.
- **Topological ordering (RR-YHGJHG)**: apply ALL entities before any relation that
references them; relation-deletes before entity-deletes. There is no batch
transaction — convergence is via per-record idempotent replay; a mid-batch
failure is recovered by re-running (resume from last good cursor).
- **Manual conflict resolution (keep dumb)**:
  - `rela sync push --force <id>` — local wins, overwrite remote, re-baseline index.
  - `rela sync pull --force <id>` — remote wins, overwrite local, re-baseline.
Force bypasses the If-Match / both-dirty guard.

## Connecting through the OAuth proxy (prod) — token form DECIDED

Prod rela-server sits behind an OAuth proxy (oauth2-proxy, per
docs/security.md). The server has NO native auth — it trusts the proxy-set
`X-Forwarded-User`. So the CLI authenticates **to the proxy, not to rela**.

- **CLI auth = JWT bearer.** oauth2-proxy supports non-browser clients via
`--skip-jwt-bearer-tokens`: it validates a JWT in `Authorization: Bearer
<token>` and injects the same `X-Forwarded-User` identity headers as a browser
session. So the client sends `Authorization: Bearer $RELA_SYNC_TOKEN` and the
server sees an authenticated, principal-stamped request. No browser redirect
needed.
  - Client: `--remote <url>` (proxy-fronted base URL, persisted per-project) +
token from `RELA_SYNC_TOKEN` env or `--token`. NEVER hardcode, NEVER log.
  - The CLI only PRESENTS a token; obtaining it (IdP service-account /
client-credentials / device flow) is out-of-band and out of scope.
- **Deployment requirements to document (operator's job, not rela code):**
proxy run with `--skip-jwt-bearer-tokens`, `--bearer-token-login-fallback=false`
(so an absent/invalid token returns a clean 403 instead of an HTML login
redirect), and `--extra-jwt-issuers` / correct `aud` if the CLI's tokens are
minted for a different client/issuer than the proxy's own (the OKTA aud-mismatch
footgun — oauth2-proxy issues #1350/#2190).
- **Origin**: server exempts `/api/sync/` from the same-origin gate (sync 4), so
the CLI needs no Origin header.
- Loopback/dev (no proxy): works without a token; principal defaults to `unknown`.

## Acceptance

- Local create/update/delete pushes to server; both ends converge (AC #1).
- Server-side create/update/delete pulls back; tombstone mirrors a local delete (AC #2).
- Concurrent edit → halt + clear report; `--force` resolves + re-baselines (AC #3).
- Relation listed before its endpoint in a batch still applies (reordered) (AC #6).
- Mid-batch failure → re-run resumes from cursor → converges (idempotent replay).
- `--force` on a non-existent id → clear error, no partial state.
- Against a proxy-fronted server, `rela sync` with `Authorization: Bearer` token
authenticates and writes attribute to the token's user; missing/invalid token →
proxy 403, surfaced clearly and distinct from 412/422. Token never appears in
logs.

## Notes

Attachments out of scope (documented limitation, RR-1IBB49). The opaque cursor
is stored/echoed verbatim — the client never parses it.
