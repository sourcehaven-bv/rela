---
id: RR-8IX47M9
type: review-response
title: 'Design: reference cost on fs and mem'
finding: Each reference call lists every row of the scope types; a reconcile run costs O(n x calls).
severity: nit
reason: Acceptable for the local backends; a batched multi-bound form is noted for step 5.
status: deferred
---
