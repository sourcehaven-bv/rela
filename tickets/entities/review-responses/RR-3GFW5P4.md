---
id: RR-3GFW5P4
type: review-response
title: A spent neoq job runs once more after the cutover
finding: The neoq adapter dropped a job that had used its budget only when neoq delivered it again, so such rows wait in neoq_jobs. The cutover gave them MaxAttempts 1, so a RetryNever job that had failed once would run a second time under River.
severity: significant
resolution: 'Addressed: neoqJobParams drops a row whose retries reach its budget. seedNeoq adds a spent RetryNever row and TestPostgresQueue_MovesNeoqJobs asserts it does not run.'
status: addressed
---
