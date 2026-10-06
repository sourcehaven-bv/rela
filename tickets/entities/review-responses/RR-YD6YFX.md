---
id: RR-YD6YFX
type: review-response
title: 'Raw path: weak aggregation and unbounded size'
finding: Per-entity paths split one route into many buckets; path length is client-controlled up to the 1 MB header limit.
severity: minor
resolution: 'Path capped at 512 bytes with path_truncated=true (test TruncatesLongPath). Route patterns deferred: the consumer (perf-report) already collapses IDs.'
status: addressed
---
