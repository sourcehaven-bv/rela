---
id: RR-1KP8CD
type: review-response
title: promote_to of a manual-ID entity type is not refused at load
finding: 'realtimecfg.promote never checks EntityDef.IsManualID, and PromoteRequest carries no id, so such a config fails only at runtime. 0.1 says no new validation is needed, but this one is: refuse it in validate.go next to the reserved promote name.'
severity: minor
resolution: validate.go refuses promote_to of a manual-ID entity type.
status: addressed
---
