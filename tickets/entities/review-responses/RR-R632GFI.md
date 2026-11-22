---
id: RR-R632GFI
type: review-response
title: Pushed sort ignores Kind and Scalar
finding: 'Code review: predicates differing only in Kind or Scalar sort unstably and give unequal scan keys.'
severity: nit
reason: Costs an extra read at most and cannot occur for one property on one type.
status: wont-fix
---
