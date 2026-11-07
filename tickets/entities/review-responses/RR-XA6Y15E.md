---
id: RR-XA6Y15E
type: review-response
title: Raw deleted state stored in job rows
finding: The payload holds the raw last state, so hidden properties stay in the job table after a hard delete.
severity: minor
resolution: TKT-RJS3KKJ seals the state in the payload under a key in state.KV that is erased when the job ends or the entity is purged; postgres sweeps finished job rows after 24 hours.
reason: Delete versions are not on every backend, so the payload is the only uniform source. The job table is tenant-internal, like the version store that already keeps the same state.
status: addressed
---
