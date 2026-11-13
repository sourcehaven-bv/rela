---
id: RR-2ZD9R0W
type: review-response
title: The engine is used as a for-loop
finding: With one branch per (from, to) flow.Decide returns NoMove both for a guard refusal and a false when:, and the definition contributes only Initial(). The plan risks being cosmetic.
severity: significant
resolution: 'Addressed in the plan: branches are selected by target through Machine.branches(from, to), the way flowcfg and realtimecfg key branches by event; the current state is read with flow.Read and a stranded value maps to ErrIllegalTransition; states with no out-edge (self-loops excluded) compile as final, and Performable returns nil for a final state through the definition.'
status: addressed
---
