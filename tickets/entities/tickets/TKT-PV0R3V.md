---
id: TKT-PV0R3V
type: ticket
title: 'Sync 4/5: server sync HTTP API (manifest + conditional push) on data-entry'
kind: enhancement
priority: medium
effort: m
status: ready
---

Sub-ticket of TKT-WE01O5 / FEAT-NJ9FEN. Addresses design-review RR-JDHDJS (sig).
Depends on sync 1 (hash), 2 (manifest/tombstones), 3 (ApplyEntity).

## Scope

Mount sync endpoints on the data-entry server **under `/api/sync/`**:

- `GET /api/sync/manifest?cursor=<opaque>` → `{ changes: { id -> hash | null },
cursor: <new opaque blob> }`. Tombstone = `null`. Server mints the opaque cursor
(internally a seq watermark; client never parses it). MVP may return full
manifest; cursor lets it become an O(changes) delta later with no client change.
- `GET /api/sync/<kind>/<id>` → full record content (for fetching a changed record).
- `PUT /api/sync/<kind>/<id>` with `If-Match: <hash>`, full content body. Applies
via `ApplyEntity`/`ApplyRelation` (sync 3). Responses kept DISTINCT:
  - `200` + new hash (applied),
  - `412 Precondition Failed` (current hash != If-Match → stale base → conflict),
  - `422` (entitymanager validation/ACL rejected the content — NOT a conflict).
- `DELETE /api/sync/<kind>/<id>` with `If-Match`.

## Auth / attribution (RR-JDHDJS) — DECIDED: reuse existing data-entry auth = OAuth proxy in prod

Prod deploys put rela-server **behind an OAuth proxy** — this is the documented,
intended model (`docs/security.md`: run behind oauth2-proxy / Vouch / traefik
forward-auth that **strips + sets** `X-Forwarded-User`; server started with
`--principal-header X-Forwarded-User`). The proxy authenticates; the app trusts
the forwarded principal header BECAUSE only the proxy can reach it. The sync
endpoints inherit this gate for free. The app itself stays auth-less by design
(`docs/security.md` 'No authentication'); a credential gate is NOT built here.

**Server-side requirements this creates:**
- The `X-Forwarded-User` (or configured `--principal-header`) value sets the sync
write's principal. Add `principal.ToolSync` (`principal.go:40-46` has none; sync
writes would otherwise mis-attribute as `data-entry`). Stamp Tool=sync.
- **CSRF/same-origin exemption decision (load-bearing):** `requireSameOrigin`
rejects any `/api/...` request with no `Origin` header → `403 origin_missing`
(`middleware_security.go:145-163`). A non-browser sync CLI sends no Origin.
Same-origin/CSRF protects against browser credential-confusion; it does NOT
apply to a non-browser, proxy-authenticated API client. **Decision:** exempt
`/api/sync/` from `requireSameOrigin` (treat it as a non-browser API surface),
while KEEPING the Host/DNS-rebinding check. Alternative (client sends an
allowlisted Origin header) is hacky — prefer the documented server-side
exemption.
- Allowlist-validate `id`/`kind` path params (reject path-traversal-shaped ids)
BEFORE the store.
- Trust-model invariant to document in godoc + `docs/security.md`: the
forwarded-principal trust holds ONLY if the server is unreachable except through
the proxy (the existing `--principal-header` trust boundary). Sync adds no new
hole, but the sync endpoint must not be exposed direct-to-network.

## Acceptance

- Manifest returns changed + tombstones keyed by opaque cursor; re-issuing the
cursor returns only newer changes.
- Push 200 / 412 / 422 each exercised and DISTINCT.
- Audit record attributes to Tool=sync + the proxy-forwarded principal.
- `/api/sync/` reachable by a no-Origin client (CSRF exemption works) but still
Host-checked; path-traversal id rejected; malformed cursor → full manifest, no
SQL error.
