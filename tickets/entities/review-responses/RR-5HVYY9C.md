---
id: RR-5HVYY9C
type: review-response
title: Lifecycle log records a state change that changed nothing
finding: Apply writes state suspended to the lifecycle log even when the tenant was already suspended.
severity: nit
reason: The entry records the request the control plane made, which is useful when tracing it. Apply returning a changed flag adds API for little gain.
status: wont-fix
---
