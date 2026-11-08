---
id: RR-E2TAPJ6
type: review-response
title: Memory cron.Stop comment overstates and concurrent Shutdown deadlocks
finding: cron v1 Stop returns once the goroutine receives the signal, not once it exits, and two concurrent Shutdown calls deadlock on the unsynchronized running flag.
severity: minor
reason: rela serializes Close on neoqQueue.mu and closed, so Shutdown never runs concurrently. Moved to TKT-TD68KKT.
status: deferred
---
