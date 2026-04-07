---
id: RR-ZXYX
type: review-response
title: Plan does not specify error response format / log signal for blocked requests
finding: Without a consistent rejection format, debugging dev-mode CORS issues becomes painful (the Vite proxy case will hit this immediately). Need a consistent body and a one-line log per rejection with the failing rule and the offending header value (truncated), so devs can diagnose in 10 seconds instead of guessing.
severity: nit
resolution: 'Plan updated: blocked requests return `403 Forbidden` with body `{"error":"forbidden","reason":"<rule>"}` and emit a single warn-level log line `security: blocked rule=<rule> host=<host> origin=<origin> path=<path>` (Origin truncated to 200 chars). Generic for production, specific enough for dev.'
status: addressed
---
