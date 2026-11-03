---
id: RR-H4MKYR
type: review-response
title: A corrupt index or shard stops every trigger permanently, with no way to recover
finding: 'internal/realtime/seen.go:59-106 and trigger.go:470-507. loadIndex and loadShard return an error on bad JSON, and run() aborts the whole round or reconcile on the first error. A torn write (crash during an FSKV Put, manual edit, truncated row) leaves `realtime/seen/index` or one shard undecodable. From then on RunRound fails, requeues 3 times and drops. RunReconcile fails every time, w.leading stays false, and every tick with pending changes runs another failing full reconcile. All rules stop, not only the one with the bad shard. TestSeenStore_CorruptIndex only pins that the error is returned. Fix: treat an undecodable index as missing (log an error; every rule rebaselines silently, which is the documented safe direction). Treat an undecodable shard as a reset of that rule only: drop it from the index so it rebaselines. Isolate per-rule shard errors so one rule cannot stop the others. Add tests for both.'
severity: significant
resolution: An index that does not decode is logged and treated as empty, so every rule baselines again silently. A shard that does not decode drops only its rule from the index; that rule baselines next round and the others carry on.
status: addressed
---
