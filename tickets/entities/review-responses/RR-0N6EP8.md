---
id: RR-0N6EP8
type: review-response
title: Unbounded per-request cost on long paths
finding: routeShape split the whole path before truncation and schemaRouteWord looped every entity type per segment, so a 1 MB path of short segments cost about a second of CPU when logging was on.
severity: significant
resolution: The word set is built once per schema snapshot (routeWordCache) and routeShape stops at maxLoggedPath. TestRouteShape_StopsAtCap bounds allocations on a 1 MB path.
status: addressed
---
