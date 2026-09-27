---
id: RR-3H5SUV
type: review-response
title: Non-lowering scoped lists now plan pushdown first
finding: Scoped lists that do not lower now run ReadQuery, the relation classifier and planListPushdown before falling back.
severity: nit
reason: CPU only; the budget tests show no extra store reads, and non-lowering scopes decline before any traversal is gated.
status: wont-fix
---
