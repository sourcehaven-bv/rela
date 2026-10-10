---
id: RR-LCFHNG
type: review-response
title: Route words missing from the allowlist
finding: _caldav, .well-known, _custom, _mcp, _lifetimes, accept, v2 and SPA client routes were missing.
severity: minor
resolution: Added, plus TestFixedRouteWords_CoverRegisteredRoutes which checks every literal segment of registered route patterns.
status: addressed
---
