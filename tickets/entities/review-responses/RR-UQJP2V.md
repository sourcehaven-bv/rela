---
id: RR-UQJP2V
type: review-response
title: Incomplete does not carry to derived collections
finding: 'A collection traversed from a capped backlinks set, or by a from: "*" step, was equally partial but carried no note.'
severity: minor
resolution: viewResult.propagateIncomplete marks derived and wildcard collections incomplete after the fixpoint. TestPropagateIncomplete.
status: addressed
---
