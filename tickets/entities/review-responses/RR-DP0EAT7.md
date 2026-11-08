---
id: RR-DP0EAT7
type: review-response
title: 'Design: co-editing WebSocket writes over a GET'
finding: The read-only rule filtered methods only. GET /api/v1/_collab/... upgrades to a WebSocket that accepts document updates when the role may update, so a leaked peer key mapped to a writing role could edit bodies.
severity: significant
resolution: 'The peer edge allows an explicit route list instead of methods: GET/HEAD on entity and relation routes (/api/v1/<plural>...), /api/v1/_schema and /api/v1/_openapi.json, and POST /api/v1/_reconcile. _collab and every other underscore route answer 403 peer_route_refused.'
status: addressed
---
