---
id: RR-W3N0OE
type: review-response
title: 'Design: wiring order impossible'
finding: autocascade.New ran before the queue existed.
severity: significant
resolution: buildAutomationWithJobs runs after buildRuntimeServices; the handler binds in finishAssembly and a registration failure fails assembly.
status: addressed
---
