---
id: RR-8X7FP31
type: review-response
title: Run gate cost grows with the number of entity types
finding: Each run read compiles one subject read query per faceless type and ORs them.
severity: minor
reason: 'Correctness first: the untyped collapse was removed for the stale-edge finding. A budget test needs the wired read paths and belongs in TKT-9IT3LN8.'
status: deferred
---
