---
id: TKT-OJL2GN
type: ticket
title: Asserted roles are inert on the production JWT gate — claims dropped at requireVerifiedJWT
kind: enhancement
priority: high
effort: s
status: review
---

The asserted-role feature (TKT-RP3X3Q) shipped with all its ACL machinery —
`principal.Verified`, `asserted_role_assignments`, `SourceAsserted`,
conditional-grant reporting — but **nothing feeds it on the production JWT
path**. A verified assertion's `org_id`/`org_slug`/`roles` are dropped before
the ACL ever sees them, so an `asserted_role_assignments` policy grants nothing
in a real deployment.

## Root cause: two PRs that each passed and collided

TKT-RP3X3Q wired claims through `dataentry.JWTPrincipalResolver`. In parallel,
TKT-RYUS3H (#1178) replaced that resolver with a fail-closed **gate**
(`requireVerifiedJWT` in `internal/dataentry/jwtgate.go`) and marked the
resolver `Deprecated`. The gate was authored against the pre-RP3X3Q world: it
calls `VerifySubject` (subject only) and stamps a plain composite literal, so
the claims RP3X3Q added never enter the request.

Both PRs merged green. Neither test suite exercised the *other's* path, so the
interaction was invisible until the branches were both on `develop`.

## Where it breaks (current develop)

- `internal/dataentry/jwtgate.go:141` — `cfg.Verifier.VerifySubject(...)` returns
the subject only; org/roles are never requested.
- `internal/dataentry/jwtgate.go:161-164` — stamps
`principal.Principal{User, Tool}`, a plain literal. Even if claims were fetched,
this drops them. This is exactly the pattern the `resolvePrincipalEntity`
comment warns against.
- `JWTGateConfig.Verifier` is typed `subjectVerifier` (`VerifySubject` only), so
the seam itself cannot carry claims.

## What is ALREADY correct (do not re-do)

- `internal/jwtauth.VerifyAssertion` exists and already fails closed via
`classify()` (ErrInvalid vs ErrKeysUnavailable) — the RYUS3H interaction was
merged correctly here.
- `resolvePrincipalEntity` (router.go:320) already rebuilds via
`principal.Verified`, so claims survive the `principal_property` substitution.
The gate is the ONLY drop point.
- `dataentry.AssertedIdentity` (router.go:408) and the `assertionVerifier`
interface (router.go:427) already exist — the deprecated `JWTPrincipalResolver`
uses them, and its sanitize-and-`Verified` projection is the logic to port.

## Scope

**In scope** — make the gate carry claims:

- Switch `JWTGateConfig.Verifier` from `subjectVerifier` to `assertionVerifier`
(`VerifyAssertion`).
- In `requireVerifiedJWT`, stamp via `principal.Verified(user, Tool, orgID,
orgSlug, roles)`, porting the role sanitize/filter from `JWTPrincipalResolver`.
- Wire the verifier through the existing `assertionVerifier` seam in
`cmd/rela-server/main.go` (an adapter or a direct method — dataentry must not
import jwtauth).
- Rework `jwtgate_test.go`'s stub to return claims and assert org/roles reach
the ACL end-to-end (extend `TestJWTGate_RouterChainOrder`).

**Out of scope**

- Fail-closed behaviour — RYUS3H owns it and it is unchanged; `VerifyAssertion`
already classifies identically.
- The deprecated `JWTPrincipalResolver` — leave it; external embedders may use
it. It already stamps claims correctly.
- Auto-provisioning an unmatched principal — TKT-0C3II2.
- Org *enforcement* — still a separate concern; org stays attribution-only.

## Acceptance criteria

1. A verified assertion carrying `roles` grants the mapped
`asserted_role_assignments` role on a request through the real gate
(`TestJWTGate_RouterChainOrder`-style, driving the actual middleware stack).
2. `org_id`/`org_slug` reach the audit log via the gate path.
3. The gate still fails closed on a JWKS outage (ErrKeysUnavailable → 401),
unchanged from RYUS3H.
4. A verified principal whose subject matches no user entity keeps its asserted
roles through the gate (AC10 parity, end-to-end).
5. The fault is regression-guarded: reverting the `Verified` stamp to a plain
literal fails a test naming the dropped claims.

## 5-whys

- **why1** (immediate): the JWT gate calls `VerifySubject` and stamps a
subject-only Principal, so org/roles never enter the request.
- **why2**: the gate (TKT-RYUS3H) was designed against the pre-assertion world
and its seam type (`subjectVerifier`) has no claims method.
- **why3**: TKT-RP3X3Q and TKT-RYUS3H were developed in parallel on the same
file region; each rebuilt the JWT identity path for its own purpose without the
other's requirement in view.
- **why4**: neither PR's test suite exercised the other's path — RP3X3Q tested
the resolver it wired, RYUS3H tested the gate it built; no test drove a verified
assertion *with roles* through the *gate*.
- **why5** (systemic): there is no single end-to-end test asserting that a
signed assertion's roles reach an ACL decision through the production middleware
stack — the one test that would have caught a collision between any two changes
to the identity path. `TestJWTGate_RouterChainOrder` proves the subject reaches
the ACL but stops short of claims.

## Prevention

The end-to-end assertion-through-gate test added in AC1/AC4 is the systemic fix:
it pins the whole identity path (verify → stamp → resolve → ACL) so a future
rework of either the gate or the resolver cannot silently sever claims again.
This is the missing test why5 identifies.
