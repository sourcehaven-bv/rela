---
id: TKT-SP3A87
type: ticket
title: Document the authenticated-everyone public-read scoping (anonymous deferred)
kind: docs
priority: medium
effort: xs
status: ready
---

Design doc §12.3, re-scoped per DEC-PEYCJZ (2026-08-19): the headline scenario
"`everyone` reads `world:published`" means every AUTHENTICATED principal — the
intranet/portal case behind the existing identity proxy.
Anonymous/public-internet read would require changing the JWT/proxy contract and
is deferred as a future feature with its own design; it no longer gates
FEAT-9CD2MX.

Remaining work (docs only, no design note needed):

1. `docs/server-security.md` (+ the docs-project guide mirror): a short
section stating the scoping — `everyone` = all verified principals; the
public-read worlds scenario assumes the proxy; anonymous access is explicitly
out of scope and would be an additive later feature (synthetic principal at the
proxy or a dedicated wiring-bound public surface).
2. Update `.ignored/pointer-design.md` §12.3 / §14 to point at DEC-PEYCJZ
(done by the architect at decision time — verify).

No code. The worlds ACL model is unchanged: read grants stay world-shaped, and
the public surface stays structurally world-bound at the wiring site (§4.4 rule
1), which is exactly what makes anonymous additive later.
