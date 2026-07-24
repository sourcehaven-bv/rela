---
id: FEAT-OQBYHD
type: feature
title: Verify signed-JWT identity from an OIDC proxy
description: 'Add a provider-agnostic identity gate that cryptographically verifies a signed ES256 identity assertion from an OIDC proxy (Pratique, oauth2-proxy, Pomerium, ...) against its JWKS and stamps the verified stable subject as principal.user. Fail-closed and exclusive: a verification failure denies the request rather than falling back to a weaker identity source.'
status: proposed
---

Today rela can read a proxy-set identity header (`--principal-header`), but that
merely *trusts* the proxy set it. This feature adds a stronger,
provider-agnostic path: verify a signed JWT assertion against the proxy's JWKS,
so a spoofed header without a valid signature does not authenticate.

Keying on the stable OIDC `sub` (an opaque user id), not email, means audit
attribution and `acl.yaml` assignments survive a user changing their email.

- `internal/jwtauth`: an ES256 + JWKS verifier; rejects non-ES256 (incl.
`alg:none`), wrong issuer/audience, and expired/unsigned tokens; auto-refreshes
the JWKS. Includes a relaxed-audience path for a future webhook receiver.
- `--jwt-issuer/-audience/-jwks-url/-header` flags (neutral `X-Auth-Assertion`
default), env fallbacks `$RELA_JWT_*`.

Verified live end-to-end with Pratique proxying in front of rela-server: a
browser-authenticated user's request carried a real signed assertion, the
resolver verified it against Pratique's live JWKS, and the write was attributed
in the audit log to the real stable subject.

## Fail-closed (TKT-RYUS3H)

The original wiring placed the JWT resolver in a chain ahead of the plain
header, so a verification failure fell **through** to the spoofable header — an
attacker-triggerable downgrade. That is now closed:

- Identity is enforced by a dedicated gate (`requireVerifiedJWT`), not a chain
entry. A verification failure denies with 401.
- `--jwt-*` is mutually exclusive with `--principal-header` and
`$RELA_DATAENTRY_USER`; configuring both is a startup error.
- The gate covers `/api/` only; the SPA shell, static assets, and the
self-authenticating IdP webhook stay reachable.

`JWTPrincipalResolver` remains for embedders with their own chain semantics but
is deprecated and no longer wired by `cmd/rela-server`.
