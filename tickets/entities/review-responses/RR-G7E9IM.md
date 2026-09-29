---
id: RR-G7E9IM
type: review-response
title: GetRelation keeps its own endpoint rule
finding: appbuild gatedGraphReader.GetRelation gates endpoints with Family rather than EndpointsReadable; it relies on GetRelation returning default-tail edges only.
severity: nit
reason: Every store's GetRelation returns only the default tail (from_face = ''); for that edge the entity-level check on both ends is exactly the 8.2 rule. Routing one edge through FilterRelations adds nothing.
status: wont-fix
---
