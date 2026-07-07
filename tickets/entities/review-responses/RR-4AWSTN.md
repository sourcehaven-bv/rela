---
id: RR-4AWSTN
type: review-response
title: Feed route lives inside /api/ inner mux (not outer like SSE) so noCache + read gate + CSRF middleware all apply
finding: 'Wiring detail with a security consequence. router.go registers SSE on the OUTER mux (mux.HandleFunc, lines 68-70) specifically to escape the reload-lock, and everything else on the `inner` mux wrapped by noCacheMiddleware and mounted at /api/. The plan says ''register GET /api/v1/_feeds/ in registerAPIV1Routes (api_v1.go)'' — which puts it on `inner`, GOOD: it then inherits attachACLRequest''s read gate and the requireSameOrigin/isCSRFExempt chain. Make this explicit in the plan and DO NOT register the feed on the outer mux (unlike SSE) — a feed is a short request, not a long-lived stream, so it needs the reload-lock + read gate, and registering it outer would bypass attachACLRequest and leak the graph. Add the router_walk_test.go probe (the codebase convention) so registration stays covered.'
severity: minor
resolution: 'Plan updated (PLAN-6LOL0Z §3): the feed route registers on the inner /api/ mux (via registerAPIV1Routes), NOT the outer mux like SSE, so it inherits attachACLRequest''s read gate + the same-origin/CSRF chain + noCache. A router_walk_test.go probe is added per the codebase convention.'
status: addressed
---
