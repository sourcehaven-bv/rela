---
id: RR-D4DY9K
type: review-response
title: ownedEdges log key and duplicated set
finding: The 'ids' log key held a count, and pending/unknown tracked the same set.
severity: nit
resolution: Log key renamed to 'sources'; one map now tracks unresolved sources and their faces.
status: addressed
---
