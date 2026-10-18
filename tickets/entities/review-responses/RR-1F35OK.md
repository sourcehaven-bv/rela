---
id: RR-1F35OK
type: review-response
title: Orphan sweep breaks the documented git-checkout restore and is not reversible
finding: The channelGuard doc says an entity restored outside the manager (git checkout or stash) gets its channels back unchanged. The A3 orphan step tombstones them after 1h and throws the handle away, so nothing ever untombstones them and they are purged at 30d. Risk 5's "reversible tombstone" is false. Either Untombstone orphan batches when the id reappears (keep the handle) or drop orphan detection on the fs tier.
severity: significant
resolution: The sweep keeps the orphan batch handle and untombstones it when the anchor id reappears before purge. Orphan batches use the type's detached retention.
status: addressed
---
