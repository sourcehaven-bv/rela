---
id: RR-0Y7BW2
type: review-response
title: Delete-policy erasure on a non-leader process waits up to an hour
finding: 'tombstoneConfirmer.ping (internal/appbuild/realtime_worker.go:127-131) only wakes the local worker. On multi-process pg, a delete committed in a non-leader is purged on the leader''s hourly tick, not promptly as section 13 of the plan says. Data covered by on_entity_delete: delete outlives the operator''s policy.'
severity: minor
resolution: The minute DueTombstones poll erases due batches within about a minute in whichever process holds the lock.
status: addressed
---
