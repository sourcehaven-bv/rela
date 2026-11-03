---
id: RR-H3O0L4
type: review-response
title: Wrong locks for memory and sqlite
finding: 'Architecture design review: pilesLocker already decides file lock unless MemFS; leases are node-local runtime state.'
severity: significant
resolution: Plan reuses that selection; on sqlite leases live in the cache-dir KV like kvpiles.
status: addressed
---
