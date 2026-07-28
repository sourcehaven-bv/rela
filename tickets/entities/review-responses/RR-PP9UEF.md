---
id: RR-PP9UEF
type: review-response
title: 412 refetch would be served from the 60s TTL cache — returns the stale entity that caused the 412
finding: 'Step 3 requires the refetch to return CURRENT server state or the merge computes against the same stale data that caused the 412. Verified: entitiesStore.fetchEntity(type, id, force=false) returns a cached entity when isCacheValid (60s TTL, stores/entities.ts:21,112-120), bypassing the network only when force===true (:115). Worse, entitiesStore.update WRITES the PATCH response into that same cache (:163-166), so during an active autosave session the entry is continuously renewed and effectively always valid. Failure: Alice PATCHes at T+0 (cache stamped T+0). Bob writes at T+5. Alice PATCHes at T+10 → 412. The retry calls fetchEntity without force; the 10s-old entry is inside the TTL → the cache returns Alice''s OWN pre-Bob entity. The merge sees theirs===base for every property, concludes ''only we changed it, keep ours'', and re-PATCHes — with no fresh ETag either (the cached path never touched the network). Either it 412s again burning all 3 attempts then surfaces a bogus conflict, or if the implementation drops If-Match on retry it silently overwrites Bob — reproducing the exact bug this ticket exists to fix, now with a merge step providing false confidence. FIX: the 412 refetch MUST use force=true; the etag-bearing read path must bypass the TTL cache entirely (a cached entity has no meaningful ETag).'
severity: critical
status: open
---
