---
id: RR-DMTLHS
type: review-response
title: Normalization would hide parity signals
finding: Normalizing ETags hides edge-filter drift; headers like X-Total-Count are unrecorded.
severity: significant
resolution: 'Plan: fix inputs, normalize only times by value, keep ETags, record six headers.'
status: addressed
---
