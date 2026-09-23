---
id: RR-RBGNO0
type: review-response
title: _position pushdown would bypass viewCondition and queryScope
finding: listPage declines pushdown when a view condition or query scope narrows the list (BUG-F1LTP1); _position carries QueryScope and would answer for the superset.
severity: critical
resolution: 'Plan: extract the whole eligibility gate into one function used by listPage and _position.'
status: addressed
---
