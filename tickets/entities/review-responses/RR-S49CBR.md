---
id: RR-S49CBR
type: review-response
title: Several schema loads per request
finding: schemaRouteWord loaded a.State() per segment.
severity: minor
resolution: routeWordCache.words takes the snapshot once per request.
status: addressed
---
