---
id: RR-VG4BPJ
type: review-response
title: Flush probe+insert must be atomic with the fenced write (same tx, after row lock)
finding: The flush's 'author differs AND hash differs' probe and its version INSERT must run inside UpdateEntity/UpdateRelation's own transaction, after the row is locked (UPDATE or SELECT ... FOR UPDATE), so the decision and the insert are atomic with the write they fence. Under store.Tx the outer tx already holds writeAdvisoryLockKey; outside Tx, per-row locking serializes back-to-back edits. Without this, two concurrent updates could both decide to flush and double-insert — content-hash dedup is advisory (SELECT-then-INSERT), not a DB constraint.
severity: significant
reason: Flush-on-author-change split into follow-up TKT-0IGI4V; this requirement is pinned there as 'Atomicity'.
status: deferred
---
