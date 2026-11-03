---
id: RR-BGNS87
type: review-response
title: A post held by a cooldown or retry delay loses its place in the FIFO queue
finding: 'internal/realtime/trigger.go postOne. A post held back by a cooldown or a retry delay is popped, and its id goes to deferred. When it falls due, process enqueues it again with put, which appends it at the tail. Under a sustained backlog, a post that has waited its cooldown then also waits behind everything queued in the meantime, which contradicts the ''first come, first served'' promise in docs/realtime.md and TestTriggers_QueueIsFirstComeFirstServed. Fix: keep the item in the queue with a not-before time and have pop skip items that are not yet due. Or, when it is enqueued again, carry the original order position or noted time into the queue order.'
severity: minor
resolution: A held back post now keeps its first noted time through deferral, and the queue is ordered by noted time, so it returns to its old place instead of the tail. TestTriggers_CooldownKeepsQueuePlace pins it.
status: addressed
---
