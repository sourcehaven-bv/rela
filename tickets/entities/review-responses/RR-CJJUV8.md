---
id: RR-CJJUV8
type: review-response
title: search.Hit and store.EntityHeader could produce a Ref
finding: loadHitHeaders and hydrateHits still build entity.Ref field by field from Hit and EntityHeader.
severity: minor
reason: Out of PR 1 scope (no call-site migration). Adding Hit.Ref() and EntityHeader.Ref() belongs with the resolver and call-site PRs of TKT-2528AB.
status: deferred
---
