---
id: RR-HVDTUKI
type: review-response
title: 'Security: relation meta visibility is not per relation type'
finding: 'Relation meta visible: is keyed by source type and face and local roles, so one flag per relation type cannot assert the reader sees all meta.'
severity: significant
resolution: 'Plan changed: the godoc allows non-nil relation Visibility only when no role of the policy has a meta visible: block for that relation type on any source type; anything else stays nil (all pending). Step 5 note: test two source types with one meta-gated.'
status: addressed
---
