---
id: RR-3NYOU8A
type: review-response
title: No backoff for failing runs
finding: A run that failed to advance stayed due at once, so 64 failing runs filled every Due round and held back the rest.
severity: significant
resolution: 'Due and Start back off a failed run: 30s doubling per failure in a row, capped at 1h, counted in Pending.Failures. TestDue_BackoffDoublesAndCaps pins it.'
status: addressed
---
