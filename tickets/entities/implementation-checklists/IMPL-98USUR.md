---
id: IMPL-98USUR
type: implementation-checklist
title: 'Implementation: POST /webhooks/idp unreachable route fix (BUG-F3ADZO)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Reproduced the bug with a router-level test before fixing: `TestWebhook_ReachableThroughRouter` fails with `200 <!DOCTYPE html>` on the old wiring
- [x] Moved `a.registerWebhookRoutes(...)` from the `inner` sub-mux to the outer `mux` in `NewRouter` (router.go), so `/webhooks/idp` is actually reachable
- [x] Verified the route now sits outside both `/api/` (so `noCacheMiddleware` and the JWT gate don't apply) and the same-origin gate — correct, since it self-authenticates a signed JWT body
- [x] Rewrote the route's doc comment: it previously claimed to live "outside /api/" while registered on the /api/-only mux — the false comment that masked the bug
- [x] Documented in `router_walk_test.go` why the webhook is excluded from the walk oracle (a non-/api/ unregistered path returns 200 SPA, not a stdlib 404, so that oracle is blind to it)

## Quality

- [x] Reproduction test is red-before / green-after
- [x] `internal/dataentry` package tests pass
- [x] Lint clean (`just lint`), `arch-lint` clean, `plimsoll` clean
- [x] Confirmed the only full-suite failure (`internal/docscapture`) is a pre-existing headless-browser timeout, identical on unmodified `develop`
- [x] Verified interaction with #1179 (JWT role claims, merged after TKT-RYUS3H): the gate still uses the `subjectVerifier` seam; this change is isolated to routing and does not touch it
