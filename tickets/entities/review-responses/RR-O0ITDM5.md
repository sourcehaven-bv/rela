---
id: RR-O0ITDM5
type: review-response
title: Queue wait counts against the attempt timeout
finding: The attempt deadline was QueuedAt plus timeout, so an attempt started late ran with almost no time and a non-idempotent job failed as timeout although the target barely ran.
severity: significant
resolution: On start the deadline is reset to now plus the timeout; the queue deadline only bounds the wait for a worker. TestJob_TimeoutCountsFromTheStart; guide and Retry.Timeout doc updated.
status: addressed
---
