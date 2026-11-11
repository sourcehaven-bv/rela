---
id: RR-BIITO81
type: review-response
title: Closing the River client would spend an attempt per running job
finding: StopAndCancel cancels running handlers; a handler that returns the context error would count as a failed attempt and a RetryNever job would be lost on every deploy.
severity: significant
resolution: 'Addressed: a failure while the queue is closing snoozes the job with JobSnooze(0) so it runs again on another process without using an attempt.'
status: addressed
---
