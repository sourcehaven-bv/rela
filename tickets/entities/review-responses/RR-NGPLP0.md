---
id: RR-NGPLP0
type: review-response
title: /api/v1/_dashboard had no route-registration probe, so an unregistered route passed CI
finding: |-
    router_walk_test.go (TestRouterWalk_AllAPIRoutesReachHandlers) exists specifically to catch routes that are handled but never registered on the mux — TKT-TLQ94B. Its own doc, NewRouter's doc, and the comments at the registration sites all say 'when registering a new route, add a probe here'. /_config and /_sidebar both have probes. The new /_dashboard did not.

    The blind spot was proven: deleting the mux.HandleFunc line for /api/v1/_dashboard and running the ENTIRE internal/dataentry suite passed. All ~400 lines of dashboard_permission_test.go pass with the endpoint completely unreachable, because every test calls app.views.handleV1Dashboard directly and bypasses the mux.

    Compounded with RR-9AECGP this was genuinely nasty: route silently unregistered → SPA gets a 404 → whole app shows the error screen → CI green.
severity: significant
resolution: |-
    Added `{http.MethodGet, "/api/v1/_dashboard", http.StatusOK}` to the probe table beside the _sidebar entry.

    Mutation-verified in the direction that matters: removing the mux.HandleFunc registration now fails TestRouterWalk_AllAPIRoutesReachHandlers/GET_/api/v1/_dashboard with 'answered by the mux's stdlib 404 — route is not registered'. The handler-level tests still pass under that mutation, which is exactly why the probe is the thing that had to exist.
status: addressed
---
