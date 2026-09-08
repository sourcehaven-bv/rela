---
id: DEC-PEYCJZ
type: decision
title: Public-read scenario re-scoped to authenticated-everyone; anonymous access deferred
context: 'Design doc §12.3 (.ignored/face-design.md): the headline worlds scenario "everyone reads world:published" seemed to require an anonymous-passthrough or public-surface mode. rela-server has no auth layer; multi-user deployments sit behind an identity proxy that verifies every principal (docs/server-security.md). Supporting anonymous readers would require changing the JWT/identity-proxy contract — a new auth-layer feature for a deployment shape that is not rela''s current target.'
consequences: The built-in everyone role means every AUTHENTICATED principal; "everyone reads world:published" is an intranet/portal scenario behind the existing proxy, needing no auth-layer change. TKT-SP3A87 shrinks from a design note to recording this decision in the server-security docs. Anonymous/public-internet read becomes a possible future feature with its own design (principal mapping, surface allowlist, rate limiting) — nothing in Steps 1-5 depends on it, and the worlds ACL model (world-shaped read grants, wiring-bound public surface) is unchanged and already accommodates it later.
date: "2026-08-19"
status: accepted
---

Decided 2026-08-19 by Jeroen (architect check-in during the FEAT-9CD2MX ticket
arc).

The alternative — an anonymous-passthrough mode — would touch the JWT/proxy
contract that every current deployment relies on, to serve a scenario
(unauthenticated public internet reads) that is not the product's present
target. Re-scoping keeps §12.3 from gating the feature: the "public" surface in
the design doc is wiring-bound `visibility(everyone) ∘ world(published)` where
`everyone` = all verified principals.

If anonymous read is wanted later it is additive: a proxy-level synthetic
principal or a dedicated public surface can be designed then without reworking
world grants, because read grants are world-shaped and the public surface is
structurally bound to its world at the wiring site (§4.4 rule 1).
