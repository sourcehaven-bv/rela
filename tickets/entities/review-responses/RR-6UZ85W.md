---
id: RR-6UZ85W
type: review-response
title: No test pins the incoming side of the scope pushdown
finding: Only an outgoing scoped case was tested, so narrowing the scoped query to one direction would pass.
severity: significant
resolution: TestCheckCardinality_ScopeCountsBothDirections covers an incoming edge from an out-of-scope tail, an outgoing edge to an out-of-scope head, and a scoped subject with no edge.
status: addressed
---
