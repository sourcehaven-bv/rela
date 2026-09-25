---
id: RR-BCBX0O
type: review-response
title: Lease is extended on child start only, not on settle
finding: The plan says each child start or settle extends the lease; only start does, widening the window in which a run with live children is reaped.
severity: minor
reason: With the StartChild claim a child of a reaped run no longer executes, so reaping a run with live children costs a retry, not a double delivery. The lease still extends on every child start, which bounds the gap between starts; a single child's duration is bounded by the 15-minute handler timeout, below the 20-minute running lease.
status: wont-fix
---
