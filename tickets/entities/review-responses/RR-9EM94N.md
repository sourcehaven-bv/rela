---
id: RR-9EM94N
type: review-response
title: Drop() keeps deferred entries, so a non-leader keeps trying the lock every tick
finding: 'internal/realtime/trigger.go:188-211 with realtime_worker.go:417-428. Drop clears pending/full but not deferred. Pending() then stays true once a deferred due time passes, so a process that is not leading calls TryLock and Drop on every 500 ms tick indefinitely. When it does get the lock it posts its stale deferred ids, which bypasses the leader''s cooldown. Fix: clear deferred (and the rest of the per-leader state) in Drop.'
severity: minor
resolution: Drop is gone. A follower runs no rounds, notes no changes and only retries the lock on its takeover timer.
status: addressed
---
