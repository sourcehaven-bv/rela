---
id: RR-QOSPDF
type: review-response
title: Unknown icon is echoed in the 400 detail
finding: service.go echoes the caller's icon string in the error detail; name the rule instead of the value (RR-I5S4NK precedent).
severity: nit
resolution: The error reads "unknown icon; see the icons list" without echoing input; owner errors likewise never carry the target. TestService_UnknownIconIsNotEchoed.
status: addressed
---
