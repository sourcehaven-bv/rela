---
id: RR-GJLLB6
type: review-response
title: validateFormRelationSide re-implemented InferDirection inline
finding: 'InferDirection was written and documented as "the single shared rule", but validateFormRelationSide kept a second copy of the same truth table inline (against relDef rather than meta). The two agreed today but would not stay agreeing: the next person adding a case changes one and misses the other, and the failure mode is silently binding the wrong side — exactly the bug class this ticket exists to prevent. You cannot claim a single shared rule and keep a second copy forty lines away.'
severity: critical
resolution: 'Fixed: validateFormRelationSide now takes *metamodel.Metamodel and calls InferDirection for an absent direction, so there is exactly one side-resolution rule in the package. Its godoc states the rule explicitly and warns against re-deriving it locally.'
status: addressed
---
