---
id: RR-PPZCOR
type: review-response
title: Bucket table is a shared resource any group can exhaust
finding: About 600 per-key buckets per identity at 300 writes/min with 2 min lifetime (one identity alone with an /h rate); about 14 accounts fill the table and every other caller gets 429. No per-identity bucket cap and no reserved room for new aggregate buckets.
severity: significant
resolution: Per-identity cap of 64 per-key buckets refuses only that identity; aggregate buckets live in a separately bounded identity space; staleness is full-based; MaxRateBuckets raised to 32768.
status: addressed
---
