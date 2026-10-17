---
id: RR-0JWVOE
type: review-response
title: Refused rate-limit takes insert buckets
finding: ratelimit.go inserts missing per-key buckets before computing the wait; a refused take keeps them, so an identity with an empty aggregate can fill the 8192-bucket table and every request needing a new bucket is refused process-wide.
severity: significant
resolution: The limiter computes refill and wait without inserting; a missing bucket counts as full and is inserted only on success. TestRateLimit_RefusedTakeInsertsNothing.
status: addressed
---
