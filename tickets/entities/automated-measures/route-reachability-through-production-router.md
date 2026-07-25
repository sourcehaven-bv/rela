---
id: route-reachability-through-production-router
type: automated-measure
title: Every registered route is reachable through the production router
description: Assert each registered route responds through app.NewRouter() (the real mux composition) rather than through the sub-mux it was registered on. Catches routes registered on a sub-mux whose mount prefix does not match the route's own path — which returns the SPA catch-all instead of the handler.
kind: test
location: internal/dataentry/router_walk_test.go
status: active
---

## Why

`POST /webhooks/idp` (BUG-F3ADZO) was registered on the `inner` mux, which is
mounted only at `/api/`. Because the route's path does not carry that prefix, it
was never reachable — requests fell through to the SPA catch-all and returned
200 HTML. The existing walk test did not catch it because it did not exercise
the composed production router from the outside.

## Measure

For every route the app registers, issue a request through `app.NewRouter()` and
assert the response is **not** the SPA catch-all. A route that returns the SPA
shell for a non-GET method, or returns HTML where JSON is expected, is
unreachable regardless of what it was registered on.

This is a structural check: it holds no matter which sub-mux a future route is
registered on, and it fails loudly rather than silently degrading to a 200.
