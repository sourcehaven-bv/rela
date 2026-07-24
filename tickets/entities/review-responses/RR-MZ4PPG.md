---
id: RR-MZ4PPG
type: review-response
title: Purge-tombstone interaction of flush must be reasoned explicitly (safe only because flush snapshots the live pre-write row)
finding: 'Sync capture (WriteVersion) already inserts outside the sweep lock by design (tx.go: a write Tx must not block on a sweep tick), so flush does not newly break purge''s mutual exclusion — but the plan''s ''dedup makes races harmless'' claim is unproven as written. After a --force-live purge, the tombstone suppresses re-capture only of the LIVE content hash. The flush is safe iff it snapshots exactly the current DB row (whose hash equals the tombstoned live hash → dedup-suppressed); if it ever snapshots stale in-memory state, it could resurrect purged content. The design must pin: flush reads the pre-edit state from the row inside the update tx, never from caller-supplied memory.'
severity: significant
reason: Flush split into follow-up TKT-0IGI4V; pinned there as 'Purge safety' (snapshot from the row inside the update tx, never caller memory).
status: deferred
---
