---
id: RR-JCH2B1
type: review-response
title: sqlite purge reads entity type and face in swapped columns
finding: internal/store/sqlitestore/purge.go liveEntityHash selected `id, face, type, ...` while scanEntity reads `id, type, face, ...`, so a --force-live purge wrote a tombstone with the wrong hash. On develop the next tick re-captured the purged content; with the column gate it was hidden until the row was saved again, then re-captured.
severity: critical
resolution: Fixed the column order. Storetest SweepBacklog/UnchangedSaveAfterForceLivePurge now force-live purges a batch of rows, saves them unchanged, and asserts only the tombstone remains on both backends.
status: addressed
---
