---
id: RR-PXM1WA
type: review-response
title: Guard coverage inconsistent
finding: trivialScopeCalls matches only the store identifier while parseRefCalls matches any receiver; aclmap and importer ParseRef sites are outside the guard (cranky review).
severity: minor
resolution: parseref now also scans internal/aclmap, internal/importer and internal/docs (importer pinned with a reason; docs moved to ParseAddress). trivialScopeCalls matches any package identifier as the receiver, like parseRefCalls.
reason: The parseref guard covers the four edge trees named in the design. aclmap and importer take operator input, not request input; widening and harmonizing both guards is follow-up work.
status: addressed
---
