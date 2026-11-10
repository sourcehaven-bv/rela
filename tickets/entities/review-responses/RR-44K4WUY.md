---
id: RR-44K4WUY
type: review-response
title: Queue pool size and ownership
finding: River needs a pgxpool.Pool and the store's pool is a pgstore.DBTX, so the queue cannot share it; its pool must still fit River's producer, notifier and workers.
severity: minor
resolution: 'Addressed: the queue opens its own pool of six connections; the decision is recorded in jobqueue_postgres.go.'
status: addressed
---
