---
id: TKT-9PCL7D
type: ticket
title: Reject reserved system:* principals at the API boundary (system:scheduler impersonation)
kind: enhancement
priority: high
effort: s
status: done
---

## Description

`principal.UserScheduler` (`"system:scheduler"`) and `principal.UserProvisioner`
(`"system:provisioner"`) are fixed, grantable identities intended **only** for
internal invocation paths — the scheduler runner and lazy provisioning. Nothing
currently stops those exact strings arriving over the HTTP API as an acting
user, so a caller who can influence the principal source impersonates the
scheduler and inherits whatever role `acl.yaml` assigns it — typically `read:
["*"]`, because that is what the DEC-O59WM4 migration writes. Reserve the
`system:` namespace at the API boundary and reject it.

## Problem

`system:scheduler` is a fixed identity for the scheduler's own invocation
(`internal/principal/principal.go:413`, DEC-O59WM4). It is grantable in
`acl.yaml`, and `internal/migration/acl_scheduler_grant.go:150` injects a
`scheduler-system` role with `read: ["*"]` so existing scheduled jobs keep
working after the migration.

Nothing rejects that string when it arrives from outside. `sanitizeUser`
(`internal/dataentry/router.go:715`) replaces control characters and caps
length; `:` is untouched and there is no prefix check of any kind. There is no
`IsSystem()` / reserved-prefix helper anywhere in `internal/principal`, and no
guard in `internal/acl`.

**The ACL match is a raw string lookup.** `Request.computeGlobals`
(`internal/acl/resolver.go:34`) does `policy.Assignments[m]` over a member set
seeded with `r.principal.User` — no namespacing, no source check, no distinction
between a principal stamped by the scheduler process and one that arrived over
HTTP. `ForPrincipal` only rejects `""` and `"unknown"` (`isUnstamped`,
`internal/acl/request.go:268`). Nothing in `internal/acl` reads `Tool` beyond
that check, so the `data-entry` vs `scheduler` Tool difference mitigates
nothing.

Every HTTP source of `Principal.User` can therefore produce the scheduler's
identity:

- `HeaderPrincipalResolver` (`internal/dataentry/router.go:468`) — trusted-proxy
header. Its own godoc states the value "is only as trustworthy as the reverse
proxy that sets it".
- `EnvPrincipalResolver` (`internal/dataentry/router.go:490`) —
`$RELA_DATAENTRY_USER`, and **highest priority** in the production chain
(`cmd/rela-server/main.go:299`).
- `verifiedPrincipal` (`internal/dataentry/router.go:606`) — the
verified-assertion projection shared by the deprecated resolver and the
production JWT gate (`internal/dataentry/jwtgate.go:161`), which serves both the
SPA/API and the remote MCP endpoint via `toolForPath`. A valid signature proves
the IdP issued the subject; it does not prove the subject is not a reserved
internal name.

So `X-Remote-User: system:scheduler`, `RELA_DATAENTRY_USER=system:provisioner`,
or a JWT `sub` of `system:scheduler` all sanitize cleanly and are stamped
verbatim. The privilege gained is not incidental: `system:scheduler` is the one
identity rela's own migration deliberately grants blanket read.

**Secondary effect — the forged name gets persisted.** Under
`unmatched_principal: provision`, `buildStubEntity`
(`internal/dataentry/provision.go:133`) writes `p.User` verbatim as the stub's
`principal_property` join key. A forged `system:scheduler` therefore lands in
the graph, after which `ResolvePrincipal` maps that raw string to a real entity.
The substitution replaces `User` so this does not itself confer the scheduler
role, but a reserved name must not become durable graph state. (Entity *ids* are
safe — `entity.ValidateID` (`internal/entity/id.go:134`) forbids `:` — so only
the property value is at risk.)

## Why this belongs at the API, not in the ACL

The ACL resolves whatever principal it is handed, and `system:scheduler`
matching its `assignments:` entry is *correct* behaviour for the scheduler. The
defect is that an **external** entry point can assert an **internal** identity.
The fix belongs where request-path input becomes a `Principal`.

This costs the scheduler nothing: it stamps its principal in-process
(`internal/scheduler`), never through an HTTP resolver, so a boundary rejection
is invisible to it. Same for `system:provisioner`
(`internal/dataentry/provision.go:74`).

## Chokepoint analysis

Sync (`sync_handler.go:53`), CalDAV (`caldav_handler.go:48`) and MCP-over-HTTP
(`mcp_http.go:19`) all mount on the inner `/api/` mux specifically to inherit
the shared middleware, and none builds its own principal — so covering the
shared path covers them. `internal/mcp/server.go:216` defers to an
already-stamped ctx over HTTP, so it needs no guard.

The subtlety: **a guard in the resolver chain alone is insufficient.**
`stampAuditPrincipal` (`router.go:743`) runs outermost on every request, but
`requireVerifiedJWT` (`jwtgate.go:170`) **overwrites** that stamp on API paths
afterwards. A check placed only in `ChainResolvers` is bypassed in JWT mode.

`sanitizeUser` (`router.go:715`) is the one function all three HTTP username
sources pass through — header, env, **and** `verifiedPrincipal`
(`router.go:620`). It is the natural single point, though it currently signals
"unusable" by returning `""`, which every caller already treats as
deny/fall-through. Planning must decide between hardening it there versus two
explicit checks (`ChainResolvers` + `verifiedPrincipal`), given the decision
below to fail loudly rather than fall through.

`webhook.go:166` builds `"webhook:" + claims.Event` directly and
`provision.go:74` intentionally stamps `system:provisioner` — both bypass
`sanitizeUser` and need explicit exemption or separate treatment.

## Decisions

1. **Reserve the whole `system:` prefix**, not just the known constants — future
`system:*` identities are then safe by construction rather than depending on
someone remembering to extend a list. Accepted risk: an existing deployment
whose IdP issues a `system:`-shaped subject would lock that user out; call it
out in the release note.
2. **Fail closed and loudly: 403 plus a security-event log** naming the source
and remote address. An impersonation attempt is signal an operator wants — most
often a misconfigured proxy. Not a silent downgrade to `unknown`, which would
turn an attempted impersonation into an ordinary-looking anonymous request. Not
a 404: per CLAUDE.md the configuration is not a secret, these constants are
documented and appear in `acl.yaml`, so concealment buys nothing and costs
operator debuggability.
3. **HTTP boundary only.** The CLI/stdio MCP trust boundary is the operator's
shell, as it is for `db migrate` and `run_as:` in `schedules.yaml`; someone at
that shell can already act as anyone.

## Scope

**In scope**

- `principal.ReservedPrefix` + `principal.IsReserved(user string) bool` in
`internal/principal`.
- Rejection covering the header, env, and verified-assertion sources — including
the JWT-gate path, which re-stamps after the chain.
- 403 + security log on rejection.
- Internal stamping sites (`internal/scheduler`, `provision.go:74`) unaffected.
- Guard the provisioning stub so a reserved name cannot be persisted as a
`principal_property` value.

**Out of scope**

- How the ACL resolves or grants `system:scheduler`.
- The migration's `read: ["*"]` scheduler grant.
- `run_as:` in `schedules.yaml`, and the CLI/stdio MCP path.

## Acceptance criteria

1. A request whose header names `system:scheduler` is rejected with 403 and does
not reach a handler as that principal, nor fall through to `unknown`.
2. Same via `$RELA_DATAENTRY_USER`.
3. Same via a **validly signed** JWT whose `sub` is `system:scheduler` — proving
the guard survives the `requireVerifiedJWT` re-stamp.
4. Any other `system:`-prefixed user is rejected identically.
5. The rejection applies on the remote MCP endpoint, not only the SPA/API.
6. The scheduler's own in-process runs are unaffected.
7. Lazy provisioning's internal `system:provisioner` write is unaffected, and no
stub is ever created with a `system:*` `principal_property`.
8. Each rejection emits one log line carrying source and remote address.
