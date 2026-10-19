---
id: RR-D1T2IW
type: review-response
title: The worker now pins one postgres connection for the life of the process while the pool minimum is still 2
finding: 'internal/appbuild/realtime_worker.go run plus pgstore.TryKeyedLock. TryKeyedLock was written for a lock taken once per run, and it takes no keyed slot. The leader now holds it until it stops. With pool_max_conns=2, half the pool is gone for good, and keyed-lock holders (up to MaxConns/2) also each pin one connection and need a second one to write. A small pool can exhaust where it did not before. Fix: make the leader lock need MaxConns >= 3, or charge it against keyedLockSlots, and update the pool-sizing guidance.'
severity: minor
resolution: TryKeyedLock now refuses a pool with pool_max_conns below 3, since the lock pins one connection for the leader's lifetime. The godoc and the realtime guide state the requirement.
status: addressed
---
