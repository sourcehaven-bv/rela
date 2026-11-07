---
id: RR-K5Y9BA1
type: review-response
title: 'Code: confusing limit_zero test case'
finding: The case passed -1 and relied on max() to make 0.
severity: nit
resolution: Validation is split into scope, range and limit tables; limit_zero passes 0.
status: addressed
---
