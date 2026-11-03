---
id: RR-SX7FRQ
type: review-response
title: 'Bulk enter is O(N^2): every leftover id is re-read and re-evaluated every tick'
finding: 'internal/realtime/trigger.go:878-880 (requeue at the cap) with 780-862 (process). When N effects exceed the 64-post cap, the remaining N-64 ids go back to pending (requeue() also ignores MaxTriggerPending). The next tick runs one Rows batch over all of them, a shard read, eval for every rule, 64 posts and another requeue. For the documented 10k import, that is about 156 ticks x 10k rows (about 1.5M row evaluations and 156 Rows queries with 10k-id IN lists) just to drain the backlog. A today() rule at midnight behaves the same. Fix: keep the computed effect queue across rounds (keyed by rule+ref, re-validated only for ids that got a new Note) instead of requeuing ids. Alternatively, requeue only the ids actually left over per rule and cap the per-round work set (for example 4x the post cap), leaving the rest in pending. Make requeue honour MaxTriggerPending by flagging the type full.'
severity: significant
resolution: Effects that post now wait in a FIFO queue per rule that persists across rounds, so leftovers are not read or evaluated again. Rows are read in chunks of 1000; TestTriggers_QueueIsNotReadAgain pins one read for a 192 id backlog.
status: addressed
---
