---
id: RR-TITU3OU
type: review-response
title: Snoozed tasks use up the candidate cap
finding: The 20-candidate cap was applied before snooze filtering, so 20 snoozed tasks hid a 21st waiting one.
severity: minor
resolution: A flow_tasks source is neither asked for First nor cut to the cap; its list is bounded by the task call. TestFlowTasks_SnoozedTasksDoNotUseTheCap.
status: addressed
---
