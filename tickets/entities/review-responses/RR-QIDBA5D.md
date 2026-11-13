---
id: RR-QIDBA5D
type: review-response
title: Reversing MinInt64 overflowed
finding: Negating an integer amount of math.MinInt64 wrapped to itself.
severity: minor
resolution: Reverse refuses an amount whose negation does not fit in int64; the balance re-check also catches it.
status: addressed
---
