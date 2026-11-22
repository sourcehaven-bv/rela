---
id: RR-0P1EN9J
type: review-response
title: 'The failed runs make each task request heavier'
finding: 'FailedTasks runs on every tasks request, and its error fails the list.'
severity: minor
reason: 'Not fixed: the query reads only runs waiting in a job state, TestFlowJob_FailedTasksBudget pins its store calls, and an error is returned rather than hidden.'
status: wont-fix
---
