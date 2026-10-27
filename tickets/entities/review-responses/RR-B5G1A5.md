---
id: RR-B5G1A5
type: review-response
title: Holder cap of 2 000 in id order defeats the demo's main scenario
finding: 'The agent-runner case is a job identity set as `owner` on every incident. Past 2 000 incidents the holder query is cut by id order, so sessions on higher ids never appear, truncated stays true for good, and no cursor reaches them. Ref-only headers are cheap, so the cap protects little. Fix: bound the TOTAL deduplicated anchor list at MaxWaitingAnchors instead of 2 000 per pair, and set truncated only past that bound. Pass GraphQuery.Limit = cap+1 so every backend orders by id and bounds in-store: graphquerynaive orders only when paged, and unpaged AllFaces rows come in store order. Without a deterministic order, the capped subset changes between calls, the watch digest flaps and cursors leave gaps.'
severity: significant
resolution: No per-pair cap. One deduplicated anchor bound of MaxWaitingAnchors over all pairs; truncated only past it. HolderLookup uses OrderBy id,face with Limit cap+1 (paged by Offset while redaction drops rows) for a stable subset.
status: addressed
---
