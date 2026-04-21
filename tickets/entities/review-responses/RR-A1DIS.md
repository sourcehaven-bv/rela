---
id: RR-A1DIS
type: review-response
title: Magic numbers 3 and 4 for hub bucket are inline
finding: '`legendTargetThreshold = 5` is named; `3` and `4` for the hub bucket are inline as `n >= 3` and the implicit `< 5`. Inconsistent. Name `minHubTargets = 3`.'
severity: nit
resolution: Added `minHubTargets = 3` as a named constant alongside `legendTargetThreshold = 5`. Both thresholds are now documented in a block comment describing the classifier's buckets.
status: addressed
---
