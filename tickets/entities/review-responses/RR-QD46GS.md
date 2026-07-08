---
id: RR-QD46GS
type: review-response
title: Page-merge must dedupe by entity ID (offset skew from concurrent writes)
finding: 'The plan concatenates page data without deduplication. Store iteration order is deterministic (fsstore: sorted entityOrder keys; pgstore: ORDER BY id ASC), but the loop issues N sequential requests against a live store: a create/delete landing between page fetches shifts offsets, so an entity can appear on two consecutive pages (duplicate) or fall between them (miss). A duplicated ID breaks Vue''s v-for :key uniqueness on the board and corrupts beginOptimistic''s find-by-id copy-on-write. Misses are self-healing (the write that caused the skew fires an SSE event that invalidates and refetches) — duplicates are not.'
severity: significant
resolution: 'Plan revised: listAllEntities merges pages into a Map<string, Entity> keyed by ID (later page wins, fresher copy), so a concurrent write shifting offsets can never produce duplicate cards. Misses self-heal via the SSE invalidation the same write triggers. Dedupe covered by a dedicated unit test (entity appearing on two pages).'
status: addressed
---
