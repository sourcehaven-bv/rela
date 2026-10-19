---
id: RR-RW2HCO
type: review-response
title: Postgres slow job row survives the conformance case
finding: The canceled updateJob rolls back, so the row stays and later subtests drop it as unroutable.
severity: nit
resolution: Not changed.
reason: Harmless log noise in the shared test schema; the existing conformance cases leave rows the same way.
status: wont-fix
---
