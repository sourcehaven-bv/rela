---
id: RR-C7I04SJ
type: review-response
title: A join whose subject is gone is polled and warns forever
finding: When the subject is deleted joinGuard fails with ErrSubjectNotFound, joinHolds warns and returns false, and joinDue schedules the next poll, with no end.
severity: minor
reason: A run whose subject is gone should end rather than be polled less. That is TKT-8137DHO (cancel runs of deleted subjects), already split from the flows work. Until then it costs one read and one Warn per orphaned run every 5 minutes.
status: deferred
---
