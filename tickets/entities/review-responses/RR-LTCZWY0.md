---
id: RR-LTCZWY0
type: review-response
title: The exemption reuses audit.FlowStep, which spreads to other writes
finding: audit.FlowStep has seven setters and is designed to flow into the automations a step triggers. Using it as authority over a run's ceiling exempts any field-gated write on a derived ctx.
severity: significant
resolution: 'Addressed in plan: a dedicated capability flowedits.WithinRun(ctx, runID) set only around the step''s grant.PatchEntity (FlowControl.complete) and the output PatchEntity (flowOutputs.Set). An archguard test pins its callers.'
status: addressed
---
