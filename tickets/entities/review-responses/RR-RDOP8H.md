---
id: RR-RDOP8H
type: review-response
title: '[security] Raw cardinality counts can reveal hidden edges'
finding: countRelations in internal/dataentry/analyze.go counts over the raw store and prints 'has N', so the difference from visible edges is the number of hidden neighbours.
severity: minor
reason: Pre-existing. Counting visible edges only is the same change as batching the counts; both are in TKT-5LW875.
status: deferred
---
