---
id: RR-4CCTJ5C
type: review-response
title: One process-wide mutex serializes index updates
finding: deletedStates.mu is held across KV round trips, so a burst of deletes serializes.
severity: minor
reason: Deletes with on.deleted background jobs are rare; a per-id lock can follow if a profile shows contention.
status: deferred
---
