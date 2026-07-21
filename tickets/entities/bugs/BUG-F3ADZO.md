---
id: BUG-F3ADZO
type: bug
title: POST /webhooks/idp is unreachable — registered on the /api/-only inner mux, falls through to SPA catch-all
description: 'The inbound IdP webhook route has never been reachable in production. registerWebhookRoutes registers POST /webhooks/idp on the `inner` mux, but `inner` is only mounted under mux.Handle("/api/", ...). Since /webhooks/idp does not match the /api/ prefix, it falls through to the SPA catch-all and returns 200 SPA HTML. Every IdP membership event has been silently dropped since #1069.'
priority: high
effort: xs
status: backlog
---

## Description

`POST /webhooks/idp` is dead code in production. It has never executed.

`registerWebhookRoutes` (`internal/dataentry/router.go:92`) registers the route
on the `inner` mux. `inner` is reachable **only** via `mux.Handle("/api/",
a.noCacheMiddleware(inner))`. The path `/webhooks/idp` does not carry the
`/api/` prefix, so Go's `ServeMux` never routes it to `inner` — it matches the
catch-all `mux.Handle("/", spaHandler(spaFS))` instead.

Notably the route's own doc comment asserts the opposite, stating it "lives
OUTSIDE `/api/`" — which is true of the *path* but not of the *mux it was
registered on*. That mismatch is likely how it passed review.

## Reproduction

Confirmed empirically with a standalone `ServeMux` program mirroring the
production wiring:

```
POST /webhooks/idp -> status 200 body "SPA"
```

Expected: the webhook handler runs and verifies the signed body. Actual: the
SPA's `index.html` is returned with 200.

## Impact

Any IdP configured to deliver membership events (`membership.created`, etc.) to
this endpoint has been receiving `200 OK` with an HTML body — so the IdP sees
success and does not retry, while rela never provisions the user. Silent data
loss with a success signal, which is the worst shape for this class of failure.

Introduced in #1069 (`feat: provision users from an IdP via signed webhooks`).

## Fix sketch

Register the route on the outer `mux` rather than `inner`. Note it must stay
outside the JWT identity gate (`requireVerifiedJWT`, added in TKT-RYUS3H): the
webhook authenticates itself by verifying a signed body against its **own**
audience and will never carry an identity assertion, so gating it would reject
every legitimate callback. `isAPIPath` already excludes it correctly.

The route also bypasses `noCacheMiddleware` once moved, which is fine for a POST
webhook.

## Test gap

`router_walk_test.go` requires a probe for every new route, but the webhook was
registered on a sub-mux the walk does not traverse from the outside. The
regression test should assert reachability through the **production** router
(`app.NewRouter()`), not through `inner`.
