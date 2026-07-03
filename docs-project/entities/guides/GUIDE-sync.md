---
audience: intermediate
id: GUIDE-sync
order: 14
status: published
summary: Two-way sync between a local fsstore project and a remote pgstore rela-server
title: Sync
type: guide
---

Two-way synchronization between a local project (`fsstore`, markdown files) and
a remote `rela-server` backed by PostgreSQL (`pgstore`). The local copy is the
working set you edit; the remote is the shared source of truth. `rela sync push`
sends your local changes up; `rela sync pull` brings shared changes down.

Sync is deliberately simple: it converges by replaying individual record writes,
detects divergence by content hash, and asks you to resolve conflicts by hand.
There is no automatic merge and no background daemon.

## Model

Each record (entity or relation) has a **canonical content hash** — a stable
SHA-256 over its normalized properties and body (see `internal/canonical`). The
hash is the sync token: identical content hashes the same on both ends, so the
client can tell whether a record changed without a server round-trip.

The client keeps a **sync index** at `.rela/sync-state.json`:

```json
{
  "records": { "TKT-1": "<hash>", "A/relates/B": "<hash>" },
  "cursor": "<opaque server token>"
}
```

- `records` maps each record's key to the hash both ends last agreed on. A local
  record whose current hash differs from its indexed hash is **dirty** and will
  be pushed. A record in the index but absent locally was **deleted** locally.
- `cursor` is an opaque watermark the server mints for the change feed. The
  client stores and echoes it verbatim — it never parses it.

A record key is the entity id, or `from/type/to` for a relation.

## Push

`rela sync push` computes the local diff against the index and, for each diverged
record, sends a conditional write:

- `PUT /api/sync/<entities|relations>/<key>` with `If-Match: <indexed-hash>`.
  - **200** → applied; the index is updated to the new agreed hash.
  - **412** → the remote changed since your indexed base → **conflict**, the
    record is halted (not applied) with a report line.
  - **422** → the server rejected the content as invalid (a validation error,
    distinct from a conflict).
- A locally-deleted record sends a conditional `DELETE` (the server rejects a
  blind delete; `If-Match` must equal the current remote hash).

Records are applied in **topological order**: entities before relations (so a
relation's endpoints exist first), and relation-deletes before entity-deletes
(so no relation is orphaned mid-batch). There is no batch transaction;
convergence comes from per-record idempotent replay, so an interrupted push
resumes correctly on re-run — the index durably records each confirmed write.

## Pull

`rela sync pull` fetches the change feed and applies remote changes locally:

- `GET /api/sync/manifest?cursor=<cursor>` returns the records changed since the
  cursor, plus a new cursor. (The same key may appear more than once; the client
  keeps the latest.)
- For each changed record:
  - remote differs from the index **and local is clean** → fetch the content and
    apply it locally via the id-preserving apply path; re-baseline the index.
  - a remote tombstone (deleted) **and local is clean** → mirror the delete.
  - remote differs **and local is also dirty** → **conflict**, halted.
  - remote equals the index → no-op.

The cursor only advances when **no conflict halted a record**, so a conflict (or
a transient transport failure) leaves the cursor where a re-run resumes.

## Conflicts

A record that changed on both ends is never merged automatically. Push/pull halt
it with a clear report and exit non-zero. Resolve it explicitly:

- `rela sync push --force <id>` — **local wins**: overwrite the remote with your
  local copy and re-baseline the index.
- `rela sync pull --force <id>` — **remote wins**: overwrite your local copy with
  the remote and re-baseline.

Force re-reads the other side's current hash and supplies it as the precondition,
so it overwrites the conflicting record without disabling the server's
"no blind writes" guard. `--force` on a record that exists on neither side is a
clear error and leaves no partial state.

## Deployment behind an OAuth proxy

In production, `rela-server` runs behind an OAuth proxy (e.g. oauth2-proxy) and
has **no native authentication** — it trusts the proxy-set `X-Forwarded-User`.
The trust-boundary invariant (see [server-security.md](server-security.md)) is that the server
is reachable **only** through the proxy.

The sync CLI therefore authenticates **to the proxy, not to rela**, by presenting
a JWT bearer token. oauth2-proxy validates it (with `--skip-jwt-bearer-tokens`)
and injects the same identity headers a browser session would:

```text
rela CLI  ──Authorization: Bearer $RELA_SYNC_TOKEN──▶  oauth2-proxy ──X-Forwarded-User──▶  rela-server
```

The CLI only *presents* a token; obtaining it (an IdP service account,
client-credentials, or device flow) is out of band. The token is read from
`RELA_SYNC_TOKEN` (preferred) or `--token` and is never logged.

**Operator configuration (proxy side):**

- `--skip-jwt-bearer-tokens=true` — accept the CLI's `Authorization: Bearer` JWT.
- `--bearer-token-login-fallback=false` — an absent/invalid token returns a clean
  403 instead of an HTML login redirect (so the CLI sees a clear auth failure,
  distinct from a 412 conflict or 422 validation error).
- Ensure the token's `aud`/`iss` match the proxy's expected audience/issuer. If
  the CLI's tokens are minted for a different client than the proxy's own, set
  `--oidc-extra-audience` / `--extra-jwt-issuers` accordingly. (Audience/issuer
  mismatch is the most common "token looks fine but is rejected" footgun.)

The `/api/sync/` routes are exempt from the server's same-origin (CSRF) check for
provably non-browser clients, so the CLI needs no `Origin` header. See the
`isCSRFExempt` / `nonBrowserExemptPrefixes` documentation in
`internal/dataentry/middleware_security.go` for how that exemption is gated and
when it retires (FEAT-ESLP).

On **loopback/dev** with no proxy, sync works without a token; the principal
defaults to `unknown`.

## Limitations

- **Attachments are not synced** — only entities and relations. Attach files
  out of band.
- **The remote must run the PostgreSQL backend.** The manifest / change feed is
  a postgres feature; against a non-postgres server the manifest endpoint
  returns 501 and `pull` reports that sync is unsupported.
- **Conflict resolution is manual** (`--force`), one record at a time. There is
  no three-way merge.
