---
id: RR-VU9Q7M
type: review-response
title: 429 handling mismatches HTTP binding
finding: 429 is a response, retry_after always 0, no sleep binding.
severity: significant
resolution: 'Plan R8: pull ends run without tag moves, push errors for RetryBounded; retry_after from header on 429/503.'
status: addressed
---
