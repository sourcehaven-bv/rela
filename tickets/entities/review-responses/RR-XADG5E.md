---
id: RR-XADG5E
type: review-response
title: Validation errors name an entities entry only by its type
finding: 'Two entities: ticket items in different groups produce identical error messages so the operator cannot tell which entry is wrong.'
severity: nit
reason: Needs NavEntitiesEntries to carry group and index context through validation and the planner. The type plus the message detail is enough to find the entry in practice; worth doing with any later change to navigation validation.
status: deferred
---
