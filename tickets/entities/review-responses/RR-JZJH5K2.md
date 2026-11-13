---
id: RR-JZJH5K2
type: review-response
title: Swallowed condition compile error gives a misleading warning
finding: compile() dropped the error of ev.Compile, so a missing program would report "no metamodel" on every write.
severity: minor
resolution: compile() keeps the error per type in trigger.conditionErr, and matchesCondition reports it as the cause. Found by both the code and the security review.
status: addressed
---
