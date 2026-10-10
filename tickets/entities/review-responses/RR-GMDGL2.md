---
id: RR-GMDGL2
type: review-response
title: Count and list decide pushdown differently
finding: ListEntities pushes down only for a rowRedactor reader; CountEntities did so whenever composeReadScope succeeded
severity: minor
resolution: CountEntities now applies the same rowRedactor condition.
status: addressed
---
