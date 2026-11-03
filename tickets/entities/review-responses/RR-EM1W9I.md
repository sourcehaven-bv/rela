---
id: RR-EM1W9I
type: review-response
title: 'Every view request recompiles all configured conditions'
finding: 'The ViewConditionFunc seam compiles every list, kanban, action and traverse condition per call; mentioned_by steps compile once per schema type.'
severity: significant
reason: 'Lists, exports and nav status already pay this per request through the same seam. Memoizing on the (cfg, meta) pointers would go stale where tests and the config editor mutate the config in place; it needs its own change across all callers.'
status: deferred
---
