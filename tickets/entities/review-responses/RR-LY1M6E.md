---
id: RR-LY1M6E
type: review-response
title: Recompute lists every open channel of session types that have no timers
finding: 'Timers.recompute (timers.go:306) loops over every type with a session, not only types with HasTimers(). It runs on every leadership change, twice per reload and every 10 minutes. On filechannels each ListByLifecycle is a tree walk, so N session types without timers cost N walks that cannot change anything. A stale hint on a type without timers is never listed, because fireDue filters on HasTimers, and the wire derives due_at. Fix: skip s == nil || !s.HasTimers() in recompute, as fireDue does, and update the AC 12 budget (one ListByLifecycle per timer type).'
severity: minor
resolution: The recompute skips session types without timers; TestTimers_Budget asserts one listing per type with timers.
status: addressed
---
