---
id: RR-RX7TFU
type: review-response
title: Hand-rolled SSE refresh duplicates the query cache
finding: useEvents already invalidates entityKeys.type(type) on entity:changed and entityKeys.root on refresh (composables/useEvents.ts:120-152); the server batches per type every 200 ms (watcher.go:374). A manual on(entity:changed) + 500 ms debounce misses refresh, adds latency and double-fetches.
severity: significant
resolution: Use useQuery with entityKeys.listParams(type, params). No manual subscription or debounce. It shares a cache entry with an open EntityList using the same params.
status: addressed
---
