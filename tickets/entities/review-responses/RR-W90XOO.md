---
id: RR-W90XOO
type: review-response
title: Prune erases LastRun for intervals over 14 days, so those tasks rerun early
finding: 'scheduler.go pruneAge (14d) is documented to exceed the longest schedule, but parseSchedule accepts any positive duration (e.g. every: 720h). Prune deletes the task record 14 days after its last run; the next tick sees no record and runs it as a first run, about every 14 days instead of every 30.'
severity: significant
resolution: The prune age is now max(14 days, twice the longest task period) (pruneAgeFor), computed once in New. Schedule.Period gives the period of each schedule kind. Pinned by TestTick_PruneKeepsLongIntervalTasks (30-day task survives a prune 20 days after its last run) and TestPruneAgeFor.
status: addressed
---
