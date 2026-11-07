---
id: RR-LNQKZDP
type: review-response
title: 'Security: no upper bound on limit and bounds'
finding: A peer could send huge bounds or limits.
severity: minor
resolution: 'Plan changed: ValidateElementRange refuses bounds over 4 KiB and limits above store.MaxRangeList.'
status: addressed
---
