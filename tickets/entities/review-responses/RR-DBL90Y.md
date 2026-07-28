---
id: RR-DBL90Y
type: review-response
title: Three channels share one whole-entity ETag — self-inflicted 412s with no second user present
finding: 'The FIFO chain serializes the three autosave channels (useAutoSave.ts:305,359,406) but serialization does not save the ETag: all three PATCH the same entity and the ETag covers properties + content + outgoing edges. Each successful PATCH returns a NEW ETag (write_handler.go:454). If a chained PATCH was enqueued holding the superseded value, it 412s. Failure: Alice types in the body (content channel, 100ms debounce on EntityDetail) and toggles a checkbox (property channel). Content PATCH runs first, succeeds, returns ETag-B; the property PATCH enqueued behind it still holds ETag-A → 412. Every interleaved two-channel edit costs an extra round-trip plus backoff, entirely self-inflicted, no second user involved — and consumes attempts from the AC7 bound that real contention needs. This is strictly worse than today. NOTE: this vindicates the FIRST clause of the useAutoSave.ts:18-20 comment (''the FIFO chain already serializes per composable instance'') — which is TRUE and load-bearing. Only the SECOND clause (SSE merge path) is false. The plan treats the whole comment as stale and would discard a correct rationale. FIX: the retained etag must be a single mutable ref updated from every PATCH response INSIDE the then-handler, never captured at enqueue time; preserve the FIFO rationale in the corrected comment.'
severity: significant
status: open
---
