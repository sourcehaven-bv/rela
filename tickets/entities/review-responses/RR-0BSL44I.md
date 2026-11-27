---
id: RR-0BSL44I
type: review-response
title: Debounce tests no longer use sseFlushInterval
finding: The debounce tests use a 1s window, so they pin coalescing rather than the production constant.
severity: minor
resolution: The sseTestBurstFlush comment says so.
status: addressed
---
