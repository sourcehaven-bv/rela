---
id: RR-EYHB61
type: review-response
title: Field conditions must be judged on the prime row
finding: Folding Require into the related query's pre conjuncts would let a lower-ranked face answer for the entity (BUG-2SKLD3).
severity: significant
resolution: Require/PickRequire are applied on the ranked outer row in SQL ((SELECT e.* FROM graphSource) e WHERE ...) and on the prime header's face in the generic fold. Conformance test world_folds_the_prime_face plus a faced field-condition case.
status: addressed
---
