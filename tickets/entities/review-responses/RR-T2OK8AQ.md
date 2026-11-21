---
id: RR-T2OK8AQ
type: review-response
title: Non-staff readers on git deployments still ask git status
finding: The capability gate covers deployments without git only. A customer on a git deployment still polls /_git/status.
severity: minor
reason: Needs a per-principal decision on who may see sync state; out of this bug's scope and harmless (the endpoint answers).
status: deferred
---
