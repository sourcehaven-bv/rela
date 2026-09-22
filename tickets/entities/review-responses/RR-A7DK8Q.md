---
id: RR-A7DK8Q
type: review-response
title: 'Self-referential types: the scope section and the Validate guard contradict each other'
finding: 'The plan''s OUT list said self-referential types are undraftable but still work if written by hand, while the Approach proposed a Validate guard refusing a step whose endpoints are not actually swapped. For a self-referential type the endpoint lists are identical in both projections, so that guard refuses precisely the case the scope section says is supported - closing the only available door. Self-referential types are also the densest source of the other hazards here: task--blocks-->task is the textbook case for both self-edges and both-directions-present pairs.'
severity: significant
resolution: 'Partially overtaken by the idempotence fix, which already refuses any type whose from-list and to-list share an entity type - a superset of the self-referential case. The contradiction is resolved in that direction: refused, not hand-authorable. The OUT list has been corrected to say so.'
status: addressed
---
