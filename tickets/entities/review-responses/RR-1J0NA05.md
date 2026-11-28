---
id: RR-1J0NA05
type: review-response
title: No raw read bound after gating
finding: Counting only gated rows left raw ne/se edge reads unbounded per parent (DoS).
severity: significant
resolution: HopCap+1 raw edges per source per step; over it fails closed as OverCap for that source; hub test added; residual recorded.
status: addressed
---
