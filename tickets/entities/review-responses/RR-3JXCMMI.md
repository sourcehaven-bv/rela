---
id: RR-3JXCMMI
type: review-response
title: Missing tests for run Update and rename races on the database tiers
finding: Only a run Create during a patch is covered; a run Update, a rename racing a run start, and the pool-saturation case are not.
severity: minor
reason: Update and the rename guard go through the same txFlowRuns path as Create, which TestOutsideEdits_RunWriteTakesWriteLock pins; the pool case is removed by construction (no in-Tx pool read). Follow-up if the path diverges.
status: deferred
---
