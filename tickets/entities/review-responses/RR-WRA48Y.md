---
id: RR-WRA48Y
type: review-response
title: 'Recompute re-lists every open channel each round while capped: O(n^2/64) reads'
finding: 'Recompute calls ListByLifecycle(typ, open, ...) which has no limit (internal/channels/store.go:196; filechannels walks the whole tree, ops.go:988-1008). Writes are capped at 64 per round, and a capped pass ''remembers the config only when a pass finishes, the next pass re-derives everything''. So arming N legacy channels (or a reload that changes a timer on N channels) costs N/64 rounds x N rows: 10k open channels = 157 full listings, about 1.6M meta.yaml reads on fs or row reads on pg, at 1s intervals. Plan change: list once per (type, config pointer), keep the pending reschedules in memory and drain them in batches of MaxTimerWritesPerRound across rounds (drop the list when the pointer changes or on Reset). Extend AC 12: with 200 channels needing a write, ListByLifecycle is called exactly once per type per config.'
severity: significant
resolution: 'Plan updated: recompute lists once per (type, config), keeps pending writes in memory and drains 64 per round; dropped on pointer change or Reset; AC 12 pins one ListByLifecycle per type for 200 writes.'
status: addressed
---
