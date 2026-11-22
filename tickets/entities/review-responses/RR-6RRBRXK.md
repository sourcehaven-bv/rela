---
id: RR-6RRBRXK
type: review-response
title: "An older binary rejects the new log entry"
finding: "checkLog of a binary without ViaFailed rejects a run log that holds a failed entry, so a rollback cannot read those runs."
severity: significant
reason: "Rela rolls forward; the PR body names the rollback limit. Keeping the failure out of the log would leave the timeline without the cause, which is the bug."
status: wont-fix
---
