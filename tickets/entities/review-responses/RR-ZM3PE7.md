---
id: RR-ZM3PE7
type: review-response
title: Decode worker goroutine outlives the timeout (leak under attack)
finding: Go image decoders don't observe context. Running decode in a worker with a context deadline means on a CPU/entropy bomb the parent returns at the deadline while the worker keeps running to completion — a goroutine + partial-buffer leak under sustained attack. go.uber.org/goleak (already a dep) would flag it.
severity: significant
resolution: 'Accept that the worker runs to completion (can''t cancel a pure-Go decoder mid-loop) but make it SAFE: (1) the required concurrency semaphore caps how many such goroutines can exist at once (bounded leak, not unbounded); (2) the DecodeConfig pixel-cap runs BEFORE spawning, so a bomb that lies about dims never starts a decode; (3) a real CPU-bomb with honest small dims is bounded by the semaphore + wall-clock. Documented explicitly in the plan (the timeout unblocks the caller, it does not kill the decode) and added a goleak-based test that N bombs leave ≤ semaphore-bound goroutines. This is the same tradeoff wazero''s loop-header interruption has; documented as accepted.'
status: addressed
---
