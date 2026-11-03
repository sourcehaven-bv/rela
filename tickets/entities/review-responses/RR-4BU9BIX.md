---
id: RR-4BU9BIX
type: review-response
title: 'Type post-filter starves a pushed limit when q also names type:'
finding: 'type=X unions with a type: clause already in q (api_v1.go:2125-2149), so Y hits take slots before the exact filter drops them. Push only when there is no type filter or the parsed types are exactly {X}; decide this in the query service, not with a third entry point.'
severity: significant
resolution: 'Plan changed: executeQueryPrefiltered takes a queryOptions{prefilters, limit, exactType}; pushdown only when sq.EntityTypes is empty or exactly {exactType}, no property filters and no sort.'
status: addressed
---
