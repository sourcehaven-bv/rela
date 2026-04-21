---
id: RR-AE716
type: review-response
title: 'Disk cache: no verification stored key matches filename hash'
finding: The entry JSON contains a `key` field, but the plan never says `readCacheFile` validates `sha256(entry.key) == filename`. A corrupted or maliciously-swapped `<sha>.json` (two valid cache files renamed to each other's paths) would be returned as a hit for the wrong key, silently poisoning memoize results.
severity: critical
resolution: 'Resolved by scope change: v1 no longer has disk persistence. Original `{persist = true}` design removed in favor of process-wide in-memory cache namespaced by script path. When/if v2 adds a disk backend, the hash-verify-on-read requirement will be part of that ticket''s design.'
status: addressed
---
