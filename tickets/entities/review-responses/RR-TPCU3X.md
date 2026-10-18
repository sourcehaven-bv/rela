---
id: RR-TPCU3X
type: review-response
title: Delete-type channels kept for detached retention when batched with a detach type
finding: retention.go:101-108 purges a batch at the max over its channels, so a whole-entity delete with a delete-type and a detach-type channel keeps the delete-type messages 30d+ (pinned by TestPurgeAfter "delete batch keeps a detach type"). docs/realtime.md says delete erases at the next sweep; split batches by policy or document the exception.
severity: significant
resolution: Filing.TypeAfter plus a partial Purge erases each type at its own time (TestFilingFor_MixedBatch, TestSweep_MixedBatchPurgesEachTypeOnTime).
status: addressed
---
