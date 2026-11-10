---
id: RR-SPUJA8O
type: review-response
title: Close puts a RetryNever job back on the queue
finding: A job whose handler Close interrupted was snoozed so it ran again elsewhere, even a RetryNever job whose side effect may already have happened. River 0.51.1 also treats any error wrapping the stop's context.Canceled as a soft stop and requeues the job even when it is cancelled.
severity: significant
resolution: 'Addressed: only a job with attempts left is snoozed. Others are cancelled with an error that does not wrap context.Canceled. TestPostgresQueue_CloseRequeuesOnlyRetryableJobs pins both cases.'
status: addressed
---
