---
id: RR-IED8VHF
type: review-response
title: Operator retry can reset the budget during backoff
finding: Retry reset waiting attempts to Failed 0, giving unlimited attempts; it also settled in-flight attempts.
severity: significant
resolution: 'Plan: Retry resets only failed attempts; a waiting attempt only moves NextAt to now; a queued or started attempt returns ErrJobRunning. All failed branches of a forked run are retried.'
status: addressed
---
