---
id: RR-SQI8GTF
type: review-response
title: A failing nav status entry does its setup twice
finding: 'cranky: navStatusInStore turns scope, identity and filter-gate errors into handled=false, so navStatusRows repeats the same calls.'
severity: minor
reason: Only an entry that already fails pays it, and the result is the same. One error path (navStatusRows) keeps the error text and class identical to before, which the degrade-and-log handling relies on.
status: wont-fix
---
