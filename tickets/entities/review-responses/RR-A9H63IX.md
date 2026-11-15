---
id: RR-A9H63IX
type: review-response
title: Batch-too-large error carries package prefix
finding: 'The 422 detail read ''llmtask: ... select fewer'', which leaks the package name and is wrong on the kanban.'
severity: nit
resolution: The message now reads 'these entities are too large to send to the AI at once; run the action on fewer entities'.
status: addressed
---
