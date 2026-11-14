---
id: RR-XACFXVB
type: review-response
title: Job timeout skips the issue job
finding: A job timeout yields result cancelled, and the report job skipped cancelled runs, so the worst slowdowns went unreported.
severity: significant
resolution: 'Addressed: the run step has timeout-minutes 85 (a step timeout is a failure); the job has 100.'
status: addressed
---
