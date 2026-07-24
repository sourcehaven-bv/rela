---
id: RR-VG4BPJ
type: review-response
title: Flush probe+insert must be atomic with the fenced write (same tx, after row lock)
finding: The flush's 'author differs AND hash differs' probe and its version INSERT must run inside UpdateEntity/UpdateRelation's own transaction, after the row is locked (UPDATE or SELECT ... FOR UPDATE), so the decision and the insert are atomic with the write they fence. Under store.Tx the outer tx already holds writeAdvisoryLockKey; outside Tx, per-row locking serializes back-to-back edits. Without this, two concurrent updates could both decide to flush and double-insert — content-hash dedup is advisory (SELECT-then-INSERT), not a DB constraint.
severity: significant
reason: This finding constrains the flush-on-author-change mechanism, which the design review recommended (RR-K781MZ) splitting out of this ticket entirely because the attribution columns + sweep stamping alone fully fix the reported symptom. No flush code exists in this PR, so there is nothing here for the finding to apply to; it is pinned verbatim as the 'Atomicity' requirement in follow-up ticket TKT-0IGI4V so the flush cannot be implemented without satisfying it.
status: deferred
---
