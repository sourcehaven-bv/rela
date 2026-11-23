---
id: RR-8D68YA8
type: review-response
title: Name of the superset lowering invites misuse
finding: 'Design review: LowerConditionConjuncts reads as exact like LowerCondition; it should be named as a prefilter, return one checked lowering and drop traversals.'
severity: minor
resolution: Named queryplan.ConditionSuperset; returns one ConditionLowering within limits, without traversals; godoc says the caller must still evaluate the program.
status: addressed
---
