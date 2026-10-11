---
id: RR-P6QW0K
type: review-response
title: Item cap counts hidden and stale refs
finding: A 409 at a visible count below 500 reveals that hidden items exist, and the user cannot clear them because the SPA only sends visible refs.
severity: critical
resolution: 'Revised after user input: no refusal on size. A pile holds at most 500 stored items and an add evicts the oldest atomically. The remaining channel (eviction counting hidden items) is documented; it never reveals which or how many.'
status: addressed
---
