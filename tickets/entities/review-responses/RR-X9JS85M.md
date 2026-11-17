---
id: RR-X9JS85M
type: review-response
title: Scheduler task contexts kept a stale read time
finding: A long-running scheduler reused the read time of its first task.
severity: significant
resolution: taskContext and taskContextAs replace the read time per task with readtime.With; TestTaskContext_FreshReadTime pins it.
status: addressed
---
