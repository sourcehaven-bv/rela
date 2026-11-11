---
id: RR-SIANBHJ
type: review-response
title: Two pinned postgres connections per process but the pool check assumes one
finding: TryKeyedLock required pool_max_conns >= 3 per lock. The flow worker lock pins a second connection next to the realtime worker's, so with pool_max_conns=3 one connection is left for all writes and requests.
severity: minor
resolution: pgstore counts held keyed locks per pool; the n-th lock needs pool_max_conns >= n + 2. TestTryKeyedLock_CountsHeldLocks.
status: addressed
---
