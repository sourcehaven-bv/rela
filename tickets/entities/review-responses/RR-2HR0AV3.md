---
id: RR-2HR0AV3
type: review-response
title: 'Code: Empty() misses an empty bounded relation range'
finding: A relation range with Lo == Hi still issued a statement.
severity: nit
resolution: Empty() is true for any bounded range with Lo >= Hi; TestPlanElementRange_EmptyBoundedRange.
status: addressed
---
