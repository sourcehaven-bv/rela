---
id: RR-MZ4PPG
type: review-response
title: Purge-tombstone interaction of flush must be reasoned explicitly (safe only because flush snapshots the live pre-write row)
finding: 'Sync capture (WriteVersion) already inserts outside the sweep lock by design (tx.go: a write Tx must not block on a sweep tick), so flush does not newly break purge''s mutual exclusion — but the plan''s ''dedup makes races harmless'' claim is unproven as written. After a --force-live purge, the tombstone suppresses re-capture only of the LIVE content hash. The flush is safe iff it snapshots exactly the current DB row (whose hash equals the tombstoned live hash → dedup-suppressed); if it ever snapshots stale in-memory state, it could resurrect purged content. The design must pin: flush reads the pre-edit state from the row inside the update tx, never from caller-supplied memory.'
severity: significant
reason: The purge-tombstone interaction it analyzes only arises when a flush inserts a pre-edit snapshot, and the flush was split to follow-up TKT-0IGI4V per RR-K781MZ — the attribution-only change in this PR adds no new version-insert paths, so purge guardrails are untouched (verified by the purge suite passing). The safety argument (flush must snapshot the live pre-write row inside the update tx so its hash equals the tombstoned live hash and dedup suppresses it) is pinned as the 'Purge safety' requirement in TKT-0IGI4V.
status: deferred
---
