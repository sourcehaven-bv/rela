---
id: RR-XA6Y15E
type: review-response
title: Raw deleted state stored in job rows
finding: The payload holds the raw last state, so hidden properties stay in the job table after a hard delete.
severity: minor
reason: Delete versions are not on every backend, so the payload is the only uniform source. The job table is tenant-internal, like the version store that already keeps the same state.
status: deferred
---
