---
id: RR-T42SFAQ
type: review-response
title: A pool of 3 silently disables one of the two workers
finding: pool_max_conns=3 used to pass TryKeyedLock. With the flow worker a second pin fails on every try, a goroutine race decides which worker never leads, and the only symptom is a Warn every 10s. The new minimum was not documented.
severity: significant
resolution: buildFlowWorker calls storeLockRoom (pgstore.CheckLockRoom) for both worker locks, so a long-running host with flows on a too-small pool fails at startup with the remedy. GUIDE-flows and GUIDE-realtime state the minimum of 4. TestTryKeyedLock_CountsHeldLocks covers CheckLockRoom.
status: addressed
---
