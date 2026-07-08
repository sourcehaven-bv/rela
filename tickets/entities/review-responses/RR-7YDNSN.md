---
id: RR-7YDNSN
type: review-response
title: Sequential page fetches add latency on large boards — parallel fetch is a known future option
finding: After page 1, meta.total makes the remaining page count known; pages 2..N could be fetched concurrently (bounded) instead of sequentially. For realistic boards (≤200 entities = 2 pages) the sequential loop costs one extra round-trip and is simpler; noting the parallel option so a future perf pass doesn't re-derive it. Sequential also composes naturally with the response-driven has_more contract.
severity: nit
reason: Realistic boards (≤200 entities) are 1-2 pages — parallelism saves at most one round-trip while complicating the response-driven has_more contract and the dedupe merge. The option is documented in the fix plan so a future perf pass doesn't re-derive it; not worth the complexity now.
status: wont-fix
---
