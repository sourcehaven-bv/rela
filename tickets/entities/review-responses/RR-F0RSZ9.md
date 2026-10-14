---
id: RR-F0RSZ9
type: review-response
title: Empty fallback config prunes shared run-state
finding: With no tasks or an invalid schedules.yaml the scheduler now runs and pruned the whole run-state store at the 14-day floor, so a monthly task could run early; on postgres every task-less server pruned the shared table.
severity: significant
resolution: maybePrune skips with no tasks or while a file fails to load, and never shrinks below the largest prune age used; TestMaybePrune_SkipsAnIncompleteConfig.
status: addressed
---
