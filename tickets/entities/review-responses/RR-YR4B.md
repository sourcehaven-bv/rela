---
id: RR-YR4B
type: review-response
title: 'Architect #9: workspace still imports automation'
finding: workspace.mayDependOn still includes automation because newWorkspace calls automation.NewEngineFromMetamodel. Could be trimmed by moving construction into entitymanager.
severity: significant
reason: Filed as TKT-IPKE. Out of TKT-IU2S scope (touches entitymanager public API).
status: deferred
---
