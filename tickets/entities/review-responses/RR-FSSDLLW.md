---
id: RR-FSSDLLW
type: review-response
title: History check reads the whole timeline
finding: useHasHistory reads the full version metadata list once per entity page view where History is offered, only to learn whether it is empty.
severity: minor
reason: Metadata only, one request, and only for readers offered History. A count or limit on /_history is an API change of its own.
status: deferred
---
