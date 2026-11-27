---
id: RR-3SU27V4
type: review-response
title: Trigger write fires no update rules
finding: A trigger-written row does not run processUpdateAutomation, but the guide said a set runs the same checks as an update.
severity: significant
resolution: Kept on purpose to stop update chains; godoc and GUIDE-flows.md now say so and list the refusal reasons.
status: addressed
---
