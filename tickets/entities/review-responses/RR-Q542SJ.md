---
id: RR-Q542SJ
type: review-response
title: Unknown mode does not deny everywhere
finding: The plan claimed the unknown mode takes the deny arm everywhere. For an unknown ACL the comment routes skip permission checks and accept writes. A typed-nil Declarative must count as a configured policy.
severity: significant
resolution: Mode godoc carries the full arm table per mode and states the unknown-mode behaviour and why it is reachable only from tests. Plan security paragraph corrected. PolicyConfigured is true for the broken policy mode; TestMode_Decisions pins every cell.
status: addressed
---
