---
id: RR-LPBH01
type: review-response
title: Timeout validated after float-to-Duration conversion
finding: 'Cranky review: a huge timeout overflows the Duration conversion and could pass the bound check.'
severity: minor
resolution: The bound is checked on the float (also rejecting NaN). A 1e300 case is in TestWatch_InvalidInput.
status: addressed
---
