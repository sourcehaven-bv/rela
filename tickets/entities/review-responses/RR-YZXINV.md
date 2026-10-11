---
id: RR-YZXINV
type: review-response
title: Scheduler refusal fails open
finding: A failed type assertion skips the startup check.
severity: minor
resolution: 'rela scheduler refuses to start when the sync-backend check cannot run. Test: TestSchedulerCmd_RefusesUncheckableWorkspace.'
status: addressed
---
