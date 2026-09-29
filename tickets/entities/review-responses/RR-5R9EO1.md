---
id: RR-5R9EO1
type: review-response
title: PolicyReader reaches into the Resolver private loader field
finding: FilterRelations called r.res.load, coupling PolicyReader to the Resolver field layout.
severity: minor
resolution: PolicyReader now keeps its own load field.
status: addressed
---
