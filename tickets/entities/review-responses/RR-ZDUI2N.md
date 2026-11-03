---
id: RR-ZDUI2N
type: review-response
title: Pending or orphan batch stuck forever when restore finds live channels
finding: 'retention.go:231-244: a pending batch whose Untombstone returns OccupiedError stays pending, Purge refuses pending, so it is never purged or restored. An orphan batch with exists=true returns before batchDue and is never erased. The restore comment at retention.go:275 ("erased when due") is false; confirm the batch on Occupied and fall through to the due check.'
severity: significant
resolution: When restore returns Occupied or ErrConflict, the sweep confirms the batch and purges it when due (TestSweep_OccupiedPendingIsConfirmed).
status: addressed
---
