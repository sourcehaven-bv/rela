---
id: RR-2L4VE4L
type: review-response
title: '[security] Integer casts could error on hidden rows'
finding: '[security] A cast in integerExpr could run before its guard, since PostgreSQL does not order AND operands, and its error would depend on a row the read gate later hides.'
severity: minor
resolution: Every guard sits in its own nested CASE, so a cast never runs on a value it cannot read.
status: addressed
---
