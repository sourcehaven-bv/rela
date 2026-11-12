---
id: RR-6X3AUZN
type: review-response
title: A failed enqueue waits out the whole timeout
finding: Saving queued then enqueueing leaves a failed enqueue queued until its deadline.
severity: minor
resolution: 'Plan: an enqueue error puts the attempt back to waiting with the Unsent backoff at once.'
status: addressed
---
