---
id: RR-MAILI1
type: review-response
title: Interval fan-out has no stable occurrence across scheduler retries
finding: The design used the scheduler run token for interval occurrences but every retry creates a new token. A partial expansion retried across a slot boundary could mint new child identities and resend completed recipients.
severity: critical
resolution: Restrict for_each to day and weekday calendar schedules in this slice. Their local-date occurrence is stable across expansion retries. Interval support requires explicit persisted slot state and is rejected at config load until then.
status: addressed
---
