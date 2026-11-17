---
id: RR-N6X26MF
type: review-response
title: rela scheduler judged grants at process start
finding: The scheduler bound every task under the read time of the root context; set once at start; so a long-running scheduler kept an expired grant alive.
severity: critical
resolution: Scheduler.readNow stamps a fresh read time per task; subscription and remote pull. TestTaskContext_StampsAFreshReadTime pins it.
status: addressed
---
