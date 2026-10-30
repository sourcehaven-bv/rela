---
id: RR-Z2UW4YA
type: review-response
title: Failed coalesced reload loses the invalidation
finding: kick marks the invalidation covered before the reload; a transient failure leaves the screen stale until the next event.
severity: minor
resolution: A failed current background reload retries with backoff (1s doubling, at most 5). Unit test added.
status: addressed
---
