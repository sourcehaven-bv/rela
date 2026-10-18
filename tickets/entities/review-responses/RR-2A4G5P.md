---
id: RR-2A4G5P
type: review-response
title: pg leader lock pins a connection and keyed slot per process, no liveness check
finding: realtime_worker.go:186-188,306 holds the blocking AcquireKeyedLock for the process lifetime. The leader and every waiter (blocked in pg_advisory_lock) each keep one pool connection and one of MaxConns/2 keyed slots forever, starving other keyed-lock users. A dropped session frees the lock server-side while the old leader keeps leading; use pg_try_advisory_lock per tick, as the pgstore sweep does.
severity: significant
resolution: Replaced by pgstore.TryKeyedLock taken per run, which uses no keyed slot, never waits, and holds a connection only during the run.
status: addressed
---
