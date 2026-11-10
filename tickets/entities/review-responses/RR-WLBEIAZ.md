---
id: RR-WLBEIAZ
type: review-response
title: Collector Flush reports a duplicate job as a failure
finding: jobs.Collector.Flush returned ErrDuplicateJob from a keyed enqueue at commit, so a committed write could look failed only because the same work was already pending.
severity: significant
resolution: 'Addressed: Flush skips ErrDuplicateJob. TestCollectorFlush_DuplicateIsNotAnError pins it.'
status: addressed
---
