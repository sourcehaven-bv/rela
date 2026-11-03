---
id: RR-0N8ILT
type: review-response
title: Quote the type in the error
finding: '''fsstore: unknown entity type: %s'' prints nothing for an empty type, which the plan lists as an edge case. Use %q, as convert.go already does.'
severity: nit
resolution: 'Plan updated: the error formats the type with %q.'
status: addressed
---
