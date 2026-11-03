---
id: RR-7FBP7DE
type: review-response
title: nextMonthOnDay duplicated rrule logic in the test
finding: A today-relative monthly case needed a helper that recomputes month lengths.
severity: nit
resolution: Removed the helper and the today-relative monthly case; added a fixed BYMONTHDAY=31 case to TestRruleNext.
reason: Replaced by a fixed BYMONTHDAY=31 case.
status: addressed
---
