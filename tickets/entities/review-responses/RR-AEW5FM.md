---
id: RR-AEW5FM
type: review-response
title: CLAUDE.md jobs rule no longer describes the scheduler's idempotency key
finding: The rule says a recurring task's key means one pending at a time. The scheduler now keys by run id and non-overlap lives in run state; a reader could 'fix' the key back to the task name.
severity: minor
resolution: 'CLAUDE.md jobs rule gains a paragraph: the scheduler keys each job by run id, non-overlap lives in internal/schedulerstate, and it must not move back into the queue key (BUG-TKL08E).'
status: addressed
---
