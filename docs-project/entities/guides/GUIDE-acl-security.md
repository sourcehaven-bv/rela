---
id: GUIDE-acl-security
type: guide
title: "ACL: Security Hardening"
status: published
order: 21
audience: advanced
summary: "Operator's hardening guide for rela's ACL system: group membership trust, fail-loud boot, audit-isolation invariants"
---

This guide covers the security properties an operator running
rela with an `acl.yaml` needs to understand. Read [GUIDE-acl-overview]
first; this assumes you know the resolver vocabulary.

## Hardening `member-of`

rela's v1 ACL confers group roles by walking `member-of` edges from
the principal. By default, `member-of` is a regular relation type —
there is no built-in restriction on who can create one. **If you use
groups in `assignments`, you must gate `member-of` writes**, or any
user who can create a relation can grant themselves any role.

The simplest hardening is to require a `member-of:create` permission
on the relation and grant it only to administrative roles:

```yaml
role_relations:
  member-of:
    requires_permission: member-of:create

roles:
  admin:
    permissions: [member-of:create]
    write: ["*"]
```

With that in place, only principals who hold `member-of:create`
(directly or via inherited role) can add someone to a group.
Operators of single-user instances who don't use groups can ignore
this; the moment you add an `assignments` mapping for a group, this
is mandatory.

The companion section in `docs/security.md` carries the same
guidance with more context on the broader threat model.

## Fail-loud on malformed `acl.yaml`

A malformed `acl.yaml` fails boot. This is intentional: silently
degrading to NopACL (allow-all) on a typo would invert the operator's
intent — they wrote a policy specifically *to* restrict access. The
operator sees a clear error referencing the parse failure and the
file path; they fix the file and restart.

A genuinely absent `acl.yaml` is different: no file means "no access
control intended," and the server boots with `acl.NopACL{}`.
`rela-server` will warn at startup if you also bind non-loopback in
this configuration (no auth + no ACL + reachable from the LAN is
almost never what you want).

## Why `nil Subject` panics

`AuthorizeWrite` panics when its WriteRequest carries a nil Subject.
This is not a security feature per se — it's a programmer-error
guard. A nil Subject in a request means a call site forgot to
construct one, and the safe-looking alternatives (silently allow,
silently deny) both lose information:

- Silently allow → bypasses the ACL entirely at a single buggy site.
- Silently deny → looks like a permission problem when it's a code
  problem; the operator wastes time tweaking the policy.

The panic surfaces the bug at the call site, where it can be fixed,
on the first request that hits it. The dataentry call sites are
exhaustively tested for Subject population so this never reaches
production traffic.

## Audit-isolation invariant on the SSE stream

The data-entry SSE event stream (`/api/events`, `/api/v1/_events`)
broadcasts `{type, id}` markers when entities are created, updated,
or deleted. It deliberately does NOT carry audit records, principal
identity, or attribution chains. A denied write produces a
`denied-write` audit row server-side (with full attribution) and
**zero** events on the SSE wire.

This separation matters because SSE is a fan-out channel — every
subscriber sees every event. Putting audit attribution on it would
leak the principal-to-entity topology to anyone connected,
including a malicious browser tab the user is unaware of.

A regression test in `internal/dataentry/sse_audit_isolation_test.go`
pins the invariant. Future work that adds new SSE event types must
preserve it; a new audit-aware channel needs to be a separate
broker with per-subscriber ACL gating.

## Deny response shape

A 403 from a denied write carries `rule_kind`, `rule_id`, and a
human-readable `reason`. It does NOT carry the principal's
attribution chain. This is by design:

- The wire body is what the *requester* sees. Telling them which
  groups they're in and via what edges would reveal organisational
  topology to a possibly-attacking client.
- The audit log is what an *operator* sees. It has the full chain.

If you're debugging "why was alice denied X?" — read the audit log
on the server, not the response body in the browser.

## Read-path gating: per-entity responses

ACL v1 (TKT-VQGN) gates every per-entity-response code path:

| Surface | Hidden-target behaviour |
|---|---|
| `GET /api/v1/<type>/<id>` | 404 with the same `not_found` body as a nonexistent id; no `ETag` header, no `If-None-Match` honoured |
| `PATCH/DELETE/POST` on `/api/v1/<type>/<id>` | 404 with the same `not_found` body, BEFORE body parse / `If-Match` / `IsLocked`; a malformed PATCH body on a hidden id returns 404, not 400 |
| `?include=*` and `?include=<type>` neighbours on any GET | hidden neighbours silently omitted from the `included` map; one batched `store.GraphQuery` per neighbour-type (not per neighbour) |

The deny shape is "indistinguishable from not-found." An attacker who
can probe URLs sees only 404 for every hidden entity, regardless of
verb. The 403 path is reserved for **visible-but-write-denied** — a
principal who can read the type but not write to that specific
record. That 403 still carries `rule_kind` / `rule_id` for
operator debugging.

### Invariants downstream maintainers MUST preserve

- **No `ETag` on deny.** Suppressing it is what closes the
  cross-principal cache poisoning surface. A future change that
  emits an `ETag` on the 404 path turns a denied principal's
  `If-None-Match: <alice-etag>` into a 304 — confirming existence.
- **All conditional-request headers short-circuit on deny.**
  `If-None-Match`, `If-Modified-Since`, `If-Match`,
  `If-Unmodified-Since`, `If-Range` MUST be consulted only AFTER
  the visibility probe passes. Today's handler emits only `ETag`,
  but the next maintainer to add `Last-Modified` needs to land the
  deny-side suppression in the same change.
- **The method dispatcher consults URL shape only.** Routing
  `GET/PATCH/DELETE/OPTIONS` for a path MUST NOT consult entity
  existence — the per-method handler is the gate. Otherwise an
  OPTIONS response shape becomes an existence oracle.
- **`?include=` uses the consumer-side `readGate`, batched per
  neighbour type.** A hub entity with 50 neighbours triggers ≤
  `O(distinct-types)` `GraphCount` calls, not 50. Future include
  surfaces (e.g. nested includes in a list response) MUST go through
  the same gate.

## ACL fail-loud and middleware scope

The `attachACLRequest` middleware:

- **Wraps `/api/` paths only.** The SPA shell at `/` and static
  assets at `/static/` `/assets/` bypass it. A misconfigured
  principal stamper that throws `ErrUnstampedPrincipal` returns
  500 on `/api/v1/...` but lets the UI keep rendering. Otherwise
  operators would be locked out of the very surface they need to
  diagnose the misconfig from.
- **Fails loud inside `/api/`.** When ACL is configured and the
  principal stamper produces an unstamped principal, the middleware
  returns 500 with `acl_unstamped_principal` rather than silently
  proceeding with no `acl.Request` attached. Silent fall-through
  was a fail-open path — every read became AllowAll because the
  read handlers couldn't tell "no ACL" from "ACL but no
  principal."

## What still leaks (deferred)

- **List endpoints** (`GET /api/v1/<plural>`) — every principal
  with reach to the URL sees every entity of the type, regardless
  of role grant. TKT-VMD8 (PR 2/2) closes this with the same
  composable `Request.ReadQuery` infrastructure the per-entity
  path uses.
- **Sidebar counts** (`listCount`, `kanbanCount`) — same; TKT-VMD8.
- **`/api/v1/_position`** — takes an `id` but resolves a scope walk,
  so it's list-derived. A hidden id appears in the scope today and
  the response leaks its ordinal. Scoped to a follow-up after
  TKT-VMD8 lands.
- **`/_search`** — the bleve / pgstore search backends need their
  own query-rewrite. Separate ticket.
- **SSE `/api/v1/_events`** — the broker today fans `{type, id}` to
  every subscriber. Per-subscriber filtering is its own ticket.
- **MCP transport** — tracked as TKT-G3PPD.

For threat-modelling purposes today: assume any principal with API
reach can enumerate every entity via the LIST endpoint (PR 2 closes
this). Per-entity GET, write, and include channels are tight.

## Where to read next

- [GUIDE-acl-overview] — operator's overview of the resolver.
- [CON-authorization] — vocabulary and core concept.
- `docs/security.md` — the broader `rela-server` threat model
  (CSRF, DNS rebinding, loopback binding, command allowlist).
