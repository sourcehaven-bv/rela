---
id: RR-F0OWC8U
type: review-response
title: $now with an offset resolved to a midnight
finding: $now+1d went through date_add and lost the time of day.
severity: significant
resolution: Offsets apply only to $today; $now+Nd stays a literal.
status: addressed
---
