---
id: RR-INR4Q5
type: review-response
title: C3-daysbetween-saturates
finding: days_between computed the span via time.Duration - an int64 nanosecond count that saturates at ~292 years rather than wrapping. 9999-01-01 minus 1000-01-01 returned 106751 days instead of 3286817. A birthdate or a zero-valued year-1 date silently produced a plausible-looking wrong number.
severity: critical
resolution: 'Compute on Unix seconds instead: (utcDay(a).Unix() - utcDay(b).Unix()) / secondsPerDay. Cannot saturate in any realistic range and both operands are midnight-aligned so the division is exact. Pinned by TestDaysBetween_NoInt64Saturation.'
status: addressed
---
