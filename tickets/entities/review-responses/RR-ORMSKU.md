---
id: RR-ORMSKU
type: review-response
title: Gated search buffered every candidate before applying the limit
finding: 'Security review: with Go-side filters the SQL carries no LIMIT, and the two-pass form drained all rows into memory where the old code streamed.'
severity: minor
resolution: Pass 1 stops once q.Limit rows are already kept; undecided rows ahead of them can only add to the kept set, so nothing later can be emitted.
status: addressed
---
