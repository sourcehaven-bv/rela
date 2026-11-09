---
id: RR-DHAPO42
type: review-response
title: Postgres goroutine test comment overclaims
finding: The postgres backend already stopped its cron, so the test does not pin this ticket's fix.
severity: nit
resolution: Comment now says it guards the listener, pool and dispatch goroutines.
status: addressed
---
