---
id: FEAT-OQBYHD
type: feature
title: Verify signed-JWT identity from an OIDC proxy
description: Add a provider-agnostic principal resolver that cryptographically verifies a signed ES256 identity assertion from an OIDC proxy (Pratique, oauth2-proxy, Pomerium, ...) against its JWKS and stamps the verified stable subject as principal.user.
status: proposed
---

Today rela can read a proxy-set identity header (`--principal-header`), but that
merely *trusts* the proxy set it. This feature adds a stronger, provider-agnostic
path: verify a signed JWT assertion against the proxy's JWKS, so a spoofed header
without a valid signature fails verification and falls through rather than
authenticating.

Keying on the stable OIDC `sub` (an opaque user id), not email, means audit
attribution and `acl.yaml` assignments survive a user changing their email.

- `internal/jwtauth`: an ES256 + JWKS verifier; rejects non-ES256 (incl.
  `alg:none`), wrong issuer/audience, and expired/unsigned tokens; auto-refreshes
  the JWKS. Includes a relaxed-audience path for a future webhook receiver.
- `JWTPrincipalResolver` in the existing resolver chain (env override -> verified
  JWT -> plain header -> unknown). Tool stays `data-entry` (the assertion changes
  who authenticated, not the entry point).
- `--jwt-issuer/-audience/-jwks-url/-header` flags (neutral `X-Auth-Assertion`
  default), env fallbacks `$RELA_JWT_*`.

Verified live end-to-end with Pratique proxying in front of rela-server: a
browser-authenticated user's request carried a real signed assertion, the
resolver verified it against Pratique's live JWKS, and the write was attributed
in the audit log to the real stable subject.
