<!-- This file is auto-generated from docs-project/entities/. Do not edit directly. -->

# ACL: Security Hardening

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

## Read-path gating is not yet ACL-enforced

ACL v1 gates writes and per-entity reads (via affordances). List
queries are not yet ACL-filtered: if a principal can see entity X
in a per-entity GET, they can also see it in a list query. This is
a deliberate v1 scoping choice; a follow-up will add list-side
filtering composed against `store.GraphQuery`.

For threat-modelling purposes today: assume any principal with read
access to a type can enumerate every entity of that type via a list
query. If you need fine-grained read isolation per entity, that
feature is not yet shipped.

## Where to read next

- [GUIDE-acl-overview] — operator's overview of the resolver.
- [CON-authorization] — vocabulary and core concept.
- `docs/security.md` — the broader `rela-server` threat model
  (CSRF, DNS rebinding, loopback binding, command allowlist).
