---
id: RR-07KHW0
type: review-response
title: Anchor limit and chunking edge cases in ListWaiting
finding: 'MaxWaitingAnchors is 20 000, but pairs × inboxHolderCap can exceed it once the config has more than 10 pairs. ListWaiting then returns ErrInvalidMessage, a 500 caused by data. With sqlite chunks of 500, the MaxWaitingRows+1 limit must apply to the merged result, not per chunk. The creator branch and the anchor branch both return a channel the caller created on an anchor they hold, so results must be deduplicated by key before counting. Fix: clamp and deduplicate anchors in realtime before the call, setting truncated (post-gate) when clamped. Deduplicate and count the merged rows in sqlchannels. Use `anchor_id = ANY($n)` on postgres instead of chunks.'
severity: minor
resolution: 'Adopted in part: realtime clamps and dedupes anchors before the call (truncated post-gate); sqlchannels merges by key and counts the merged set. Postgres keeps chunks, not = ANY: shared ?-placeholder SQL and Dialect has no array binding.'
status: addressed
---
