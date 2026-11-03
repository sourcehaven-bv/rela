---
id: RR-GKGG01
type: review-response
title: Within one rule, sorted id order can starve high ids under sustained load
finding: 'internal/realtime/trigger.go:494-500 and 868-888. Ids are processed in sorted order and the cap leftovers are requeued, so the next round sorts them together with new arrivals. If one rule keeps producing more than its share of 64 posts per tick from low-sorting ids, a high-sorting id (for example TKT-Z...) can wait indefinitely. Fix: order each rule''s queue by first-noted time (FIFO), or rotate the start position per round.'
severity: minor
resolution: Ids are processed oldest noted first and each rule's queue is FIFO across rounds, so new low sorting ids cannot overtake old leftovers. TestTriggers_QueueIsFirstComeFirstServed pins it.
status: addressed
---
