---
id: RR-G98A7GA
type: review-response
title: Generic aggregates load every row into Go
finding: sqlrecords has no Aggregator, so postgres and sqlite page every row of every owner through AggregateGeneric, without bound.
severity: significant
reason: AggregateGeneric now refuses more than MaxAggregateRows (1,000,000) rows with a 400, so the memory use is bounded. A native SQL Aggregator for postgres and sqlite is tracked in TKT-EV8U8T4.
status: deferred
---
