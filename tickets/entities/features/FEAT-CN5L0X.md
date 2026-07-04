---
id: FEAT-CN5L0X
type: feature
title: Inbound signed-webhook receiver and outbound request-signing primitives
description: Two general capabilities — a signed-JWT webhook receiver that dispatches a named Lua action, and crypto primitives (HMAC/SHA) so an action can sign an authenticated outbound HTTP request. Both provider- and use-case-agnostic; IdP user-provisioning ships as an example Lua action, not in the core.
status: proposed
---

Companion to the signed-JWT identity feature (FEAT-OQBYHD): that verifies identity
on the *request* path; this adds a *webhook* path plus the crypto primitives a Lua
action needs to call back out. Both additions are generic — nothing about users or
any particular proxy is in the compiled core.

**1. Inbound signed-webhook receiver.** `POST /webhooks/idp` verifies a signed-JWT
webhook body, then dispatches a named Lua action with the verified claims. The
receiver has no knowledge of what the action does.

- `internal/jwtauth`: a `WebhookVerifier` reusing the identity verifier's JWKS +
  issuer but pinning its OWN audience — a signed identity assertion can't be
  replayed as a webhook and vice versa (confused-deputy guard).
- `internal/dataentry` shim: verify -> dedup by jti -> dispatch under a
  `webhook-receiver` principal. Outside `/api/`; CSRF-immune by construction (a
  signed body, not a cookie), so no same-origin gate. A local `WebhookClaims` +
  wiring-layer adapter keep `dataentry` from importing `jwtauth`.
- `-webhook-audience` / `-webhook-action` flags (`$RELA_WEBHOOK_*`).

**2. Outbound request-signing primitives.** A `crypto.*` Lua binding (`sha256_hex`
+ `hmac_sha256_base64`) plus a `rela.now_unix` value, so a Lua action can sign an
authenticated outbound HTTP request to any HMAC-authenticated API — with no crypto
in the scripting layer. Verification stays in Go (a root of trust); only
client-side signing is exposed.

**Example, not core:** `examples/idp-sync.lua` demonstrates both together — a
verified membership webhook triggers an action that signs a Pratique operator-API
request and upserts a `person` keyed on the stable `sub`. The whole
person/Pratique-specific surface is those ~90 lines of Lua; swap the script to
target a different proxy or task, no rebuild.

Proven by a test that runs the shipped `idp-sync.lua` against a stub API which
verifies the Lua-assembled signature byte-for-byte, plus crypto known-vectors and
a Go-vs-Lua HMAC cross-check.
