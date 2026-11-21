---
id: RR-FNSF2U9
type: review-response
title: The tenant status endpoint reads the control table twice
finding: GET /api/v1/_tenant now does a second control-table read for plan and entitlements.
severity: nit
reason: The status endpoint answers before a tenant is acquired, so there is no lease to read from. One indexed read on a rarely called endpoint.
status: wont-fix
---
