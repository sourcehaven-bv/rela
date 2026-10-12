---
id: RR-HO56WW
type: review-response
title: Route guard overclaims coverage
finding: Keys collapse sub-routes, ignore methods, and SSE/CalDAV/export routes are never probed.
severity: significant
resolution: Guard keys on method plus full route pattern over ~69 probes incl. SSE, CalDAV, export, restore, _actions and relations/{type}/{to}; only git and SSE are exempt, with reasons.
status: addressed
---
