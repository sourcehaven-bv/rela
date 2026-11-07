---
id: RR-D9BC823
type: review-response
title: Retention DELETE is unbatched
finding: One DELETE over a large backlog holds locks and bloats WAL.
severity: significant
resolution: Deletes run in batches of 1000; TestPostgresQueue_RetentionSweepBacklog sweeps 4000 rows.
status: addressed
---
