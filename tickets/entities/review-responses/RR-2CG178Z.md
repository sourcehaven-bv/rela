---
id: RR-2CG178Z
type: review-response
title: 'when: must short-circuit before condition:'
finding: matchesWhenConditions evaluates the condition only after every when clause passes. A guard that evaluates both would bind traversals more often and add warnings.
severity: significant
resolution: 'Plan: the guard evaluates the when clauses first and returns false on the first that fails, then the condition. The when-before-condition automation in the synthetic golden pins it.'
status: addressed
---
