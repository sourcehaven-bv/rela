---
id: RR-I7SR53
type: review-response
title: tailOfExistingEdge swallowed read errors before the affordance gate
finding: '[security] tailOfExistingEdge returned the zero face on a ListRelations error, and that tail now feeds the affordance gate.'
severity: minor
resolution: tailOfExistingEdge returns (Face, error); the PATCH and DELETE relation handlers answer 500 before the gate.
status: addressed
---
