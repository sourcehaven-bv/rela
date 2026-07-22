---
id: TKT-RYUS3H
type: ticket
title: JWT identity must fail closed, never downgrade to --principal-header
kind: enhancement
priority: high
effort: m
status: done
---

## Description

When both `--jwt-*` and `--principal-header` were configured, the resolver chain
in `wirePrincipalResolvers` placed the JWT resolver ahead of the header
resolver. A JWT that failed verification returned a zero Principal, so
`ChainResolvers` fell **through** to the plain header — silently replacing a
cryptographically verified identity with a spoofable, proxy-trusted one.

The code documented this as an attacker-exploitable downgrade and mitigated it
with a startup `slog.Warn`. That was insufficient: the downgrade happens
per-request, long after startup, and is invisible in normal operation. Anyone
able to disrupt JWKS reachability — network egress, DNS, an IdP outage —
converted rela from verified identity to trusted-header identity, and rela kept
serving as if nothing had changed.

## Delivered

- **JWT identity is exclusive.** A verification failure denies; it never falls
through to `HeaderPrincipalResolver`.
- **Mutual exclusion is a startup error**, not a warning. `--jwt-*` with
`--principal-header`, or with `$RELA_DATAENTRY_USER`, refuses to boot.
Partially-set `--jwt-*` flags — which previously *silently disabled* identity,
leaving a server the operator believed was authenticating — are also refused.
- **A dedicated gate, not a chain entry.** `requireVerifiedJWT`
(`internal/dataentry/jwtgate.go`) owns verify-and-decide. `PrincipalResolver`
keeps its single-return signature: an error channel exists to answer "try the
next resolver?", and exclusivity deleted that question.
- **Error taxonomy.** `jwtauth.ErrKeysUnavailable` (JWKS unreachable, operator
fault) is distinguished from `ErrInvalid` (assertion evaluated and rejected,
client fault). Both deny. Logging splits by who must act: Debug for an absent
assertion, Info for a bad one, rate-sampled Error for a JWKS outage.
- **JWKS refresh alerting.** `RefreshErrorHandlerFunc` was available in
`keyfunc.Override` but unwired; it now carries the operational alert, at most
once per 10-minute refresh interval.
- **Header-only mode is unchanged** — verified live; it still fails open exactly
as before.

## Scope decision: `/api/` gating

The 401 gate covers `isAPIPath` only, extending the reviewed RR-T15E invariant
that `attachACLRequest` already relies on. The SPA shell and static assets stay
reachable so a client can render a signed-out state and a misconfiguration does
not lock operators out of the recovery surface; those routes serve no entity
data, and every API call the SPA makes is still gated. The bare `/api` is
included explicitly (RR-P2M7). The self-authenticating IdP webhook is
deliberately outside the gate.

## The ticket's open question, answered

The ticket asked whether the JWKS client caches keys and tolerates a brief fetch
failure. **It does.** In `jwkset@v0.11.0/storage.go:255-292`, `KeyReplaceAll` is
reached only after fetch + status + decode all succeed, so a failed refresh
leaves last-known-good keys intact. Confirmed empirically: with the JWKS server
killed, a valid assertion still returned 200. No caching work was required.

**However**, a failure mode the ticket did not anticipate: on an unknown `kid` —
i.e. key rotation — jwkset performs a *synchronous* refresh bounded by the
request context (`RateLimitWaitMax: 5s`). Once fail-closed, a JWKS outage
*during rotation* denies those requests after up to a 5s stall. This is
documented in `docs/server-security.md` as an explicit availability trade-off,
with guidance to stage rotations so a new key is picked up before the old is
withdrawn.

## Verification

Unit tests (table-driven), race detector clean, `just lint` / `arch-lint` /
`plimsoll` / `coverage-check` all pass. Verified live against a real ES256 JWKS
server: startup refusal for each conflicting config; 200 with a valid assertion
and 401 for absent / expired / forged-signature / malformed / spoofed
`X-Forwarded-User` / bare `/api` / SSE; 200 for the SPA shell; and 200 for a
valid assertion with the JWKS server killed (cached keys).
