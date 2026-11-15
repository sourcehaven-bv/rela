---
id: RR-J94BJTM
type: review-response
title: Reconcile drops renamed query indexes before building the new ones
finding: Renaming every query index makes each tenant drop and rebuild them with a plain CREATE INDEX on first open, blocking writes and leaving queries without the index in between; an old process in a rolling deploy drops them again.
severity: minor
reason: Same behaviour as the earlier face-key rename and out of scope for this ticket. Tracked as TKT-J7R3HK6.
status: deferred
---
