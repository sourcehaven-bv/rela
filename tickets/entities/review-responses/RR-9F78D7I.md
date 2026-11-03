---
id: RR-9F78D7I
type: review-response
title: Formatting relies on ES2023 Intl string input
finding: Older engines convert the string to a Number and may throw for more than 20 fraction digits.
severity: nit
reason: The supported browsers implement it; capping digits at 20 is a follow-up.
status: deferred
---
