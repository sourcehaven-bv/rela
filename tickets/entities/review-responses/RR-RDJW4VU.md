---
id: RR-RDJW4VU
type: review-response
title: Integer lowering reaches count rollups
finding: Rollup count properties are integer. The stored row holds no rollup value, so lowering a comparison on one would answer from nothing. Refuse rollup properties inside LowerCondition.
severity: significant
resolution: LowerCondition declines any comparison on a rollup property. Rollups are also absent from predicate record types, so this guard is defence in depth.
status: addressed
---
