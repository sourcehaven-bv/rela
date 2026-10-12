---
id: RR-91WUHP
type: review-response
title: filechannels byRev reads whole channel before the page limit
finding: 'byRev collects every message with rev > AfterRev across all segments, sorts, then truncates to Limit. after_rev=1 on a large channel holds the whole channel in memory per request. Fix: bounded heap or ordered segment walk that stops at Limit.'
severity: significant
resolution: byRev keeps only the best Limit candidates in a heap.
status: addressed
---
