---
id: RR-AOH7CV
type: review-response
title: Named dashboard names missing from route words
finding: buildRouteWords in requeststats.go did not add c.Dashboards keys so the access log collapsed /dashboard/<name> paths.
severity: significant
resolution: Added maps.Keys(c.Dashboards) to buildRouteWords; TestNamedDashboard_RouteWords covers it.
status: addressed
---
