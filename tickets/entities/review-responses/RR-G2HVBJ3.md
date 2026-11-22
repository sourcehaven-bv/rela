---
id: RR-G2HVBJ3
type: review-response
title: scanKey is order-sensitive for the where tree
finding: 'Design review: a or b and b or a do not share a scan.'
severity: nit
reason: Costs one extra read at most, never a wrong answer; documented on scanKey.
status: wont-fix
---
