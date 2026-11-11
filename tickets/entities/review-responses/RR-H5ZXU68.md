---
id: RR-H5ZXU68
type: review-response
title: neoq cutover can block on a running neoq process
finding: Moving neoq jobs while an older process still runs could either run a job twice or queue DROP TABLE behind the old process's row locks and stall startup.
severity: significant
resolution: 'Addressed: the cutover selects with FOR UPDATE NOWAIT under lock_timeout 5s and defers to the next start when locked. TestPostgresQueue_NeoqCutoverWaitsForLockedJobs pins it.'
status: addressed
---
