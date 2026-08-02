---
id: RR-DR2DR7
type: review-response
title: maxConcurrent = GOMAXPROCS has no ceiling; slow uncancellable decodes stall throughput
finding: 'Two related points. (a) maxConcurrent is derived from GOMAXPROCS with a floor of 1 but no ceiling; on a 64-core box the transient memory budget is 64 × 256 MiB = 16 GiB, not meaningfully ''bounded''. (b) Because a timed-out decode keeps its semaphore slot until the uncancellable decode finishes, a few slow-decoding inputs can tie up all slots and stall every subsequent Acquire (observed: fuzzer froze at 0 exec/sec). Latency-amplification soft-DoS. Cap maxConcurrent at a small constant and document the queueing consequence.'
severity: minor
resolution: Capped maxConcurrent at a constant ceiling (4) so the memory budget stays flat across core counts (no longer 16 GiB on a 64-core box). Documented the timeout/slot-holding latency-amplification consequence on MemoryBudgetBytes().
status: addressed
---
