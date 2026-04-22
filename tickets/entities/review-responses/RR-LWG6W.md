---
id: RR-LWG6W
type: review-response
title: waitForServer probes root, not /api/v1/_config
finding: Reference fixture probes /api/v1/_config — static SPA is ready before API routes are wired. Probing root gives false 'ready' and the first API call can 503/404.
severity: significant
resolution: waitForServer now probes `${url}/api/v1/_config` with the matching Origin header, so readiness means the API is live, not just static assets.
status: addressed
---
