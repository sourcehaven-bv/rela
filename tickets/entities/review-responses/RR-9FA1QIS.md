---
id: RR-9FA1QIS
type: review-response
title: Baseline reason for inbox untested_denial is inaccurate
finding: The shared reason listed input errors as refusals, omitted the 403 no_principal refusal, and named limit/kinds for the summary route, which ignores its query.
severity: significant
resolution: Each entry has its own reason. It names 403 no_principal and the Go test that covers it. Only the inbox entry mentions the 400s.
status: addressed
---
