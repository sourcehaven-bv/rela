---
id: RR-ASVKR1T
type: review-response
title: POST runs the endpoint check twice
finding: gateCreateRelationAffordances and validateRelationsModern both run checkRelationEdges on create; the goldens show one more batched header read.
severity: minor
reason: One bounded batched read per create; the golden pins its cost. Threading the result through writeCreateRelations would widen two signatures for no behavior change.
status: wont-fix
---
