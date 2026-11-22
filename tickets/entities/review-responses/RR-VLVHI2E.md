---
id: RR-VLVHI2E
type: review-response
title: absentValue indexes Args[0] unchecked
finding: A malformed Cond could panic before ValidateCond.
severity: nit
resolution: absentValue checks that a not has exactly one argument and refuses otherwise.
status: addressed
---
