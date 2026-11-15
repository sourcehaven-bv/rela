---
id: RR-65XJ4M1
type: review-response
title: Field check relies on the conjunction shape rather than UsesWhole
finding: 'security: lowerViewCondition treats EntityAttributes as complete, which holds only because ConditionConjunction admits no whole-record call except related(...).'
severity: nit
resolution: 'Addressed: a comment at the check states the dependency and what a widened lowering must add.'
status: addressed
---
