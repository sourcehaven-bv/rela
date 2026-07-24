---
id: RR-4OJAC1
type: review-response
title: Version dedup has no DB uniqueness backing — flush inserts outside the sweep lock can double-insert
finding: 'Sweep dedup is SELECT-then-conditional-INSERT, safe only because the single sweeper holds sweepAdvisoryLockKey. A flush insert without that lock can race a sweep tick and land a duplicate (entity_id, content_hash) version row; ListVersions would show two identical entries. Options: (a) flush acquires sweepAdvisoryLockKey for probe+insert (also restores purge''s mutual-exclusion assumption — preferred by the reviewer, but a blocking lock in the write path needs latency thought), or (b) a partial unique index with ON CONFLICT DO NOTHING. Must be settled before flush is implemented.'
severity: significant
reason: Flush split into follow-up TKT-0IGI4V; pinned there as 'Dedup backing' (advisory-lock vs partial unique index to be decided in its planning).
status: deferred
---
