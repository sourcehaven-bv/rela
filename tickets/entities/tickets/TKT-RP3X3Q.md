---
id: TKT-RP3X3Q
type: ticket
title: Surface org_id and roles from verified identity assertions
kind: enhancement
priority: medium
effort: m
status: in-progress
---

Extract `org_id`, `org_slug` and `roles` from the verified ES256 identity
assertion into `principal.Principal`. Asserted **roles** become grantable in
`acl.yaml` so policy can express role-based rules rather than per-user
assignments; **org** is carried for audit attribution only. Verified-JWT path
only; `--principal-header` and direct-loopback keep working with empty
org/roles.

## Problem

`internal/jwtauth.VerifySubject` (verifier.go:89-99) parses the assertion into a
`jwt.MapClaims` and returns **only** `sub` — every other claim is discarded with
the local map. `JWTPrincipalResolver` (dataentry/router.go:384-407) then builds
`principal.Principal{User, Tool}` at L405, so a request that arrived behind
Pratique carrying a verified active org and role set is reduced to a username.

`OrgID` exists in the JWT layer only on `jwtauth.WebhookClaims`
(verifier.go:107) — the webhook path, not the identity path.

Consequence: `acl.yaml` can only match on `Principal.User`. Roles must be
restated per-user in `Policy.Assignments`, or derived from graph membership
edges — duplicating an authority Pratique already maintains and signs.

## Ground truth: Pratique claim shapes

From `pratique/internal/signer/signer.go:186-217` (the minting code — the doc
sample at `pratique/docs/04-architecture.md:108-124` is stale and omits four
claims):

| Claim | Type | Presence |
|---|---|---|
| `sub` | string | always (`usr_…` / `svc_…`) |
| `email` | string | always (key present; `""` for service accounts) |
| `org_id` | string | always |
| `org_slug` | string | always |
| `roles` | array of strings | always; guaranteed `[]`, never `null` |
| `principal_type` | string | always (`user`/`app`/`pat`/`service`) |
| `client_id`, `scope`, `act` | string/string/object | omitted when unset |

Load-bearing facts:

- **Roles are bare names** (`["admin","billing"]`), scoped to the ONE active
org on the session — never global, never namespaced, never multi-org. Permission
expansion stays server-side in Pratique and is never in the token.
- **A user with no active org never receives an assertion at all**
(`pratique/internal/app/app.go:405-407` — orgless sessions resolve as
unauthenticated and redirect to tenant selection). So `org_id: ""` cannot arrive
on the verified path. AC2's absent-claims case is real for the header/loopback
paths, not for a genuine Pratique assertion.
- **`roles` is nil-guarded** at `pratique/internal/app/adapters.go:92-95`.
- **Assertion TTL is 9 minutes** (`signer.go:22`, hard-coded). Pratique's cache
key includes roles, so a role change mints fresh on the next proxied request —
but an assertion already handed downstream stays valid until `exp`. **Rela can
act on stale roles for up to 9 minutes after a role change in Pratique.** This
is an accepted cost upstream; faster revocation is not something the assertion
provides.

**Transport finding (noted, not addressed here):** Pratique sends the assertion
as `Authorization: Bearer <jwt>` (`pratique/internal/proxy/proxy.go:323`) and
strips any inbound `Authorization` first. There is no `X-Auth-Assertion` header
anywhere in Pratique — rela's `--jwt-header` default. The deployment must
therefore already set `--jwt-header` explicitly. Out of scope; recorded so the
next person does not rediscover it.

## Scope

**In scope**

- A typed assertion-claims projection in `internal/jwtauth` alongside the
existing `VerifySubject` (reusing `stringClaim`, verifier.go:167, plus a
string-array helper for `roles`).
- `OrgID`, `OrgSlug`, `AssertedRoles` on `principal.Principal`.
- Asserted roles grantable in ACL: a new `Policy` key mapping claim value →
declared role, a new `SourceKind`, and one block in `computeGlobals`.
- Threading through the resolver chain, the `resolvePrincipalEntity` re-stamp
(router.go:280), and the audit sanitiser.

**Out of scope**

- **Org matching / enforcement in `acl.yaml`.** `internal/acl` has no denial
primitive — evaluation is additive union semantics (declarative.go:24-26), first
matching role wins and is never reconsidered; and
`RoleDef.Create/Update/Delete/Read` are plain `[]string` with no `when:`. "Deny
unless org matches" inverts the core model and needs its own design. Org is
carried on the Principal and lands in the audit log; nothing evaluates it.
Separate ticket.
- **Auto-provisioning a user entity for an unmatched principal** → TKT-0C3II2.
- The `--principal-header` path gaining org/roles (no verified source).
- The webhook path (`WebhookClaims` already carries `OrgID`, nested in `data`).
- The `Authorization: Bearer` transport question above.
- `principal_type` — Pratique's own middleware does not surface it; if rela
needs to distinguish a PAT from an interactive user that is its own ticket.

## Acceptance criteria

1. A verified assertion carrying `org_id`/`org_slug`/`roles` yields a Principal
whose OrgID, OrgSlug and AssertedRoles match the claims.
2. A verified assertion with `roles: []` yields a valid Principal with empty
AssertedRoles — no error, no fallthrough. Same for absent keys (defensive:
Pratique always sends them, other proxies may not).
3. `--principal-header` requests resolve exactly as today: same User, empty
org/roles, no behaviour change.
4. Direct-loopback (no flags) requests resolve exactly as today.
5. An `acl.yaml` mapping an asserted claim value to a declared role grants that
role, attributed to a distinguishable new source kind.
6. An asserted claim mapping to an **undeclared** role is dropped silently at
resolution, matching the existing `Assignments` guard (resolver.go:36-38).
7. Audit records carry the new fields, sanitised the same way `User`/`Tool` are
(audit/filesystem.go:204).
8. `NopACL` and `ReadOnlyACL` behaviour is unchanged (both are attribution-free
by design, acl.go:118-122 — expected to need no edits).
9. `rela acl map` reports asserted-role grants without silently under-reporting
them (see risk below).
10. A verified principal with asserted roles but **no matching user entity**
keeps its asserted-role grants and is not denied outright — see the
unmatched-principal decision below.

## Decision: unmatched principal → anonymous

**Decided (2026-07-21, with the user).** A cryptographically verified principal
whose subject does not resolve to a `user_entity_type` entity stays
authenticated and keeps its asserted roles. It simply has no resolved user
entity, so everything keyed on that entity does not apply: local roles via
`role_relations`, ancestry via `inherit_roles_through`, and `principal_property`
lookups all no-op.

Rationale: asserted roles are cryptographically verified and mapped through an
operator-authored allowlist in `acl.yaml`. Their trustworthiness does not depend
on a graph entity existing, so denying outright would discard a valid, verified
grant for a bookkeeping reason. The alternative — hard-denying — makes the first
request of every SSO JIT-provisioned user fail, which behind an OIDC proxy is a
routine flow, not an edge case.

**This is a deliberate interim position.** Auto-provisioning, reconciliation and
operator visibility for unmatched principals are deferred to **TKT-0C3II2**,
which also records why the anonymous fallback is unsatisfying long-term (audit
attribution degrades; the condition is silent; IdP/graph membership drifts).

Implementation consequence: `isUnstamped` (acl/request.go:189) gates on
User/Tool and must keep hard-denying a genuinely unstamped principal — the
change here is that a *verified* principal is not unstamped merely because its
subject has no entity. Whichever behaviour lands must be pinned by an explicit
test so it cannot drift silently.

## Design notes

- **Asserted roles fit the existing idiom well.** `EveryoneRole`
(acl/resolver.go:45-47) is the precedent for a role entering the effective set
without a graph walk. The new block sits adjacent, uses the same `add` closure
and the same role-declared guard. Everything downstream — `decideFromAttrs`,
`readQuery`, `grantsPermission`, `affordances/resolver.go:565-620`, `access.go`
— is source-agnostic and absorbs it for free.
- **Do not overload `Policy.Assignments`.** Its keys are matched against
`members`, which are entity IDs from a graph walk. A claim value colliding with
an entity ID would silently grant across dimensions — a privilege escalation
vector, and unauditable. Separate key, mirroring how `role_relations` is
separate despite also producing roles.
- **`knownPolicyKeys` (acl/policy.go:302-312) is enforced by a reflection
parity test** (`policy_parity_test.go:15-30`) — a new `Policy` field without an
allowlist entry fails CI.
- **`Source` must stay comparable** — it is a map key via `attrKey`
(source.go:129-132) and mirrored into `aclmap.Route`, also a map key
(`mergeRoutes`, whocan.go:98-112). No slices on `Source`.
- **Adding a `SourceKind` is compiler-silent** — every switch has a default, so
a missed entry renders `"unknown"` and sorts at priority 999. Five hand-
maintained tables in `source_test.go` need entries; `sourceKindPriority`, both
`String()` methods and `lessSource` all need edits. Worth adding a reflective
guard. Note `TestSourceKindString_UniquePrefixes` (source_test.go:236)
constrains naming.
- **`internal/principal`'s doc explicitly gates growth on an ACL ticket** — this
is that ticket; record the justification there.

## Risks

- **`rela acl map` under-reports.** `aclmap/enumerate.go:46-64` enumerates the
principal universe from `Assignments` keys ∪ membership ∪ role-relations. A role
granted by an asserted claim has **no enumerable principal**. Mitigation: report
asserted grants once, globally, mirroring `EveryoneGrants` (access.go:74-80) —
the same pattern the everyone role uses for the same reason. Covered by AC9.
- **Trust boundary.** Nothing in `internal/acl` verifies anything; `Principal`
is stamped by entry-point middleware and trusted absolutely. Asserted roles must
be populated **only** after signature verification. A header trusted the way
`X-Forwarded-User` is trusted would be a full auth bypass. Needs an explicit
section in `docs/acl-security.md`.
- **The anonymous fallback is silent** (see the decision above). An operator
cannot currently tell that verified people are transacting without a user
entity. Accepted for now; TKT-0C3II2 owns the fix.
- **9-minute stale-role window** (above) — document it; do not try to fix it
here.
- **Audit wire format changes.** `audit/audit.go:91` embeds `Principal`
directly, so new fields land in the audit log format, and
`audit/filesystem.go:204-205` sanitises only `User`/`Tool`.

## Prior art

- Extends FEAT-OQBYHD (verified-JWT resolver) from `sub`-only to the full
identity assertion.
- Pratique's own downstream contract is `pratique/pkg/middleware/middleware.go:36-42`
— its `Identity` exposes exactly `Subject`, `Email`, `OrgID`, `OrgSlug`,
`Roles`, which is the field set this ticket mirrors.
- Follow-ups carved out of this ticket: **TKT-0C3II2** (auto-provisioning an
unmatched principal) and the org-enforcement ticket.
