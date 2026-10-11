---
id: RR-M343BQ
type: review-response
title: Old generation keeps writing records during a save's migration
finding: Record writes and the scheduler on the serving generation could run while the candidate migrated the same store, so a write could land in the old shape after the migration passed it.
severity: critical
resolution: Activator.Pause freezes record writes (503 + Retry-After), waits for in-flight writes, and halts the scheduler until resume or retirement. TestPause_HoldsRecordWrites.
status: addressed
---
