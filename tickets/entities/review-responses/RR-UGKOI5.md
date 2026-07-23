---
id: RR-UGKOI5
type: review-response
title: No global export concurrency limit in v1
finding: Many simultaneous exports each spawn a subprocess + temp dir. Per-call timeout and output cap exist, but there is no global concurrency limit, so a burst could spawn many converters at once.
severity: minor
reason: v1 targets fast converters (pandoc) and the per-call timeout+cap bound each invocation; a global semaphore is a straightforward v2 addition alongside the async job model for slow converters. Documented as a known bound in the plan risks.
status: deferred
---
