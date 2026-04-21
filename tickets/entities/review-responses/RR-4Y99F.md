---
id: RR-4Y99F
type: review-response
title: Mutex docstring claim inaccurate (round 2)
finding: Cache doc said 'every path mutates lastAccess on read' but misses, nil-deletes, and TTL-expiry-deletes do not touch lastAccess. Under heavy read-miss contention, the sync.Mutex-on-get serializes without a real reason. Either switch to RWMutex with atomic lastAccess or correct the justification.
severity: significant
resolution: Corrected the Cache docstring to acknowledge misses and expiry-deletes skip lastAccess; documented that an RWMutex+atomic swap is a defensible optimization but wants a benchmark first. Did not ship the optimization since no benchmark driving it yet.
status: addressed
---
