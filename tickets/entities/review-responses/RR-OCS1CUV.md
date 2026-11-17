---
id: RR-OCS1CUV
type: review-response
title: Non-canonical date strings reach the database backends
finding: ParseDateValue accepts RFC3339 with offsets on a date property, and legacy rows can hold TBD; byte order disagrees with Go instants and Nil binding.
severity: critical
reason: 'Not reachable in this ticket: no date comparison is pushed to postgres or sqlite, so a non-canonical stored string is judged in Go after the read. Canonicalizing stored dates is a precondition of exact lowering, tracked in TKT-4Z2BIRG.'
status: deferred
---
