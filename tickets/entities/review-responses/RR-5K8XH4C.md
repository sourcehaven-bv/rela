---
id: RR-5K8XH4C
type: review-response
title: Test does not mix active and finished runs
finding: The test reads one cancelled run, so views never sees active and finished headers in one call.
severity: minor
reason: 'The sample flow is per_subject: one, so one subject cannot hold an active and a finished run at once; the active path is pinned by TestFlowControl_RunViewAndActions.'
status: wont-fix
---
