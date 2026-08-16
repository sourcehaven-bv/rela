---
id: RR-7GYJ60
type: review-response
title: Orphaned state entries for removed/renamed tasks are never pruned
finding: runDueTasks only reads state by names in the current config, and nothing removes entries for tasks deleted from schedules.yaml. Pre-change this leaked one stale timestamp per dead task; now it leaks up to three entries, and renaming a task mid-backoff leaks the old name's rows while the new name starts clean. loadState has the config available and could prune.
severity: minor
resolution: 'Added pruneOrphanedState, called from loadState: entries in any of the three maps whose task is no longer in schedules.yaml are dropped and logged. Guarded against a nil config so it cannot wipe state when there is nothing to prune against. Covered by TestPruneOrphanedState and verified live — a seeded removed-task was pruned from all three maps with an INFO line.'
status: addressed
---
