---
id: RR-2TPR7KP
type: review-response
title: Cap meaning differed per backend
finding: pgstore capped per candidate, the generic path per chunk; one heavy source cleared a whole column.
severity: significant
resolution: 'One rule everywhere: per-source cap on gated rows.'
status: addressed
---
