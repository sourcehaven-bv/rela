---
id: RR-MAQQ2AQ
type: review-response
title: Relation address check needs a real parse
finding: Checking the relation type by substring can mis-attribute; addresses over MaxRangeBound cannot be used as bounds.
severity: minor
resolution: checkAddress splits a relation address on -- into exactly three parts and checks the type is in the span; an entity address must be type/id of a span type.
status: addressed
---
