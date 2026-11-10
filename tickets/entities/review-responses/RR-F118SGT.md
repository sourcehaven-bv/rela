---
id: RR-F118SGT
type: review-response
title: Entity panel reloads on every run change
finding: FlowRunsPanel reloaded SubjectRuns on every _flow-run change in the project.
severity: minor
resolution: The panel folds a burst of change events into one reload after 300 ms. The event names no subject, so filtering per subject is not possible.
status: addressed
---
