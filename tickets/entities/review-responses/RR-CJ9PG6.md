---
id: RR-CJ9PG6
type: review-response
title: Generic predicate SQL cannot use the proposed expression index
finding: The current scalar/list CASE in pgstore equality does not match a scalar properties expression index. Reconciliation could create indexes that the pushed query never uses.
severity: significant
resolution: Plan now introduces an explicit scalar string equality predicate shared by pushdown and index planning with a live EXPLAIN acceptance test.
status: addressed
---
