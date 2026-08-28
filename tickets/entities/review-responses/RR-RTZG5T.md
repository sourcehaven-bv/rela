---
id: RR-RTZG5T
type: review-response
title: AC2 (single atomic PATCH) contradicts the AC4 deferral fallback
severity: significant
resolution: 'AC4 marked non-deferrable: atomicity of an accepted proposal is a correctness property not a queue optimization. Risk 2 rewritten.'
status: addressed
finding: Risk 2 offers shipping a sequential pump and deferring merging, reasoning only about decision 3. But AC2 requires one PATCH carrying both the trigger property and the properties_unset. Without merging that is two PATCHes, and the window between them is a state the user never approved - if the second fails, the half-applied state persists. Atomicity of an accepted proposal is a correctness property, not a queue optimization. Either make AC4 non-deferrable or restate AC2 and document the window.
---
