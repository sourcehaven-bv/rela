---
id: RR-CK9WBAK
type: review-response
title: No test of the superset claim
finding: 'Design review: nothing tested that the partial filter keeps every row Go matches for conditions with unlowerable conjuncts.'
severity: significant
resolution: 'TestConditionSuperset_KeepsEveryMatch: randomized differential mixing lowerable and unlowerable leaves; checks the subset claim, exactness when the whole condition lowers, and that it narrows at least as far as ConditionPrefilters. Mutation-checked.'
status: addressed
---
