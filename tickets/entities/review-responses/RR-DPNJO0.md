---
id: RR-DPNJO0
type: review-response
title: The clock-jump clamp needs no durable write; make its read-time nature deliberate
finding: 'During planning I claimed the clock-jump guard (scheduler.go:333-341) mutates NextRetry in memory and is never persisted, and flagged it as a gap. That was WRONG: after `retryAt = now` the following `!now.Before(retryAt)` is trivially true, so executeTask always runs and recordSuccess/recordFailure persist the correction incidentally. The correction also does not NEED a durable write — it is idempotent and re-applied on every read, which is exactly the ''correctness must not depend on a sweeper'' property userstate documents. Do not add a SetNextRetry method to make it durable; that widens the interface for no benefit and invites two writers to disagree. Instead make the read-time nature explicit: a pure clampRetry(rs, now) function in the new package, applied after the bulk load, documented as operating on untrusted input (hand edits, VM snapshot resume, NTP step).'
severity: minor
resolution: 'No Store method is added for the clamp. It becomes a pure clampRetry(rs, now) in the new package, applied by the scheduler after Load, documented as operating on untrusted input and as idempotent by design — so it must not depend on having been written back. This also records the correction: my planning claim that the clamp is ''never persisted'' was wrong, since after retryAt = now the following !now.Before(retryAt) is trivially true and the execution that follows always records state.'
status: addressed
---
