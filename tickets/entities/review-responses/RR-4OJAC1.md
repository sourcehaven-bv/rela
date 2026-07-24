---
id: RR-4OJAC1
type: review-response
title: Version dedup has no DB uniqueness backing — flush inserts outside the sweep lock can double-insert
finding: 'Sweep dedup is SELECT-then-conditional-INSERT, safe only because the single sweeper holds sweepAdvisoryLockKey. A flush insert without that lock can race a sweep tick and land a duplicate (entity_id, content_hash) version row; ListVersions would show two identical entries. Options: (a) flush acquires sweepAdvisoryLockKey for probe+insert (also restores purge''s mutual-exclusion assumption — preferred by the reviewer, but a blocking lock in the write path needs latency thought), or (b) a partial unique index with ON CONFLICT DO NOTHING. Must be settled before flush is implemented.'
severity: significant
reason: Applies only to the flush-on-author-change insert path, which was split out of this ticket per RR-K781MZ — this PR adds no version inserts outside the sweep's advisory lock, so the dedup race it describes cannot occur in the shipped code. Pinned as the 'Dedup backing' requirement in follow-up TKT-0IGI4V, where its planning must choose between acquiring sweepAdvisoryLockKey and a partial unique index with ON CONFLICT before any flush lands.
status: deferred
---
