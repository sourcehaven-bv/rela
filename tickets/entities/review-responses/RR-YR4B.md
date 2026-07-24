---
id: RR-YR4B
type: review-response
title: 'Architect #9: workspace still imports automation'
finding: workspace.mayDependOn still includes automation because newWorkspace calls automation.NewEngineFromMetamodel. Could be trimmed by moving construction into entitymanager.
severity: significant
resolution: Resolved by the workspace-decomposition arc rather than the proposed
  trim - internal/workspace was deleted entirely, so the workspace.mayDependOn
  arch-lint entry and the offending automation import no longer exist. Engine
  construction now lives in appbuild (automation.NewEngineFromMetamodel), which
  is the composition root and is allowed to depend on automation. TKT-IPKE was
  closed as obsolete in the 2026-07-20 backlog sweep for the same reason.
status: addressed
---
