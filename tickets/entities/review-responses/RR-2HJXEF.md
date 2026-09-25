---
id: RR-2HJXEF
type: review-response
title: Graceful shutdown counts as a task failure; docs say abandoned
finding: On shutdown the handler ctx is cancelled and finish records failed via WithoutCancel, not abandoned; docs/scheduled-tasks.md says otherwise.
severity: minor
resolution: docs/scheduled-tasks.md now says a run cut off by shutdown is recorded as failed and retried on the ladder, and a run still queued at shutdown is abandoned when its lease expires.
status: addressed
---
