---
id: RR-8H2GS8
type: review-response
title: 'Code/Security: foreground mode leaks the saver ctx into the job'
finding: context.WithoutCancel kept the saver's acl.Request, world, deferral and audit label; queue mode had no bound request so transition guards failed closed.
severity: significant
resolution: Jobs start from context.Background (foreground) or the queue ctx, carrying only loop guards, and bind a fresh acl.Request for the job identity. TestAutomationJobs_ForegroundIdentity.
status: addressed
---
