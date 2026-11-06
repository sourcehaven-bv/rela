---
id: RR-MNE7RMD
type: review-response
title: Reused id lets the gate decide on the new row
finding: GateDeleted gates against the store by id. A restore or a recreated id before the job runs makes the new row decide the row gate and redaction for the old state.
severity: significant
resolution: subject() refuses the job when the face is stored again, checked before and after the gate. Tested in TestAutomationJobs_DeletedForeground.
status: addressed
---
