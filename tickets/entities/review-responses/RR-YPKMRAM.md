---
id: RR-YPKMRAM
type: review-response
title: Capture jobs snoozed on processes without cards
finding: On a shared durable queue a process without an evaluator snoozes capture jobs and logs a warning, for example during a rolling deploy.
severity: nit
reason: 'Accepted design from the design review (RR-U55X4FQ): only a process that can compute cards takes the kind.'
status: wont-fix
---
