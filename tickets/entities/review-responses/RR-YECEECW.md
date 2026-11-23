---
id: RR-YECEECW
type: review-response
title: Superset filter can exceed store Cond limits
finding: 'Design review: conjuncts that each fit can together exceed store.MaxCondLeaves, so the store refuses the query and Evaluate fails.'
severity: significant
resolution: queryplan.ConditionSuperset adds a conjunct only while withinCondLimits holds for the AND; TestConditionSuperset_StaysWithinCondLimits pins it with store.ValidateCond.
status: addressed
---
