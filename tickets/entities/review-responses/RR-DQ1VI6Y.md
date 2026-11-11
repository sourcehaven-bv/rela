---
id: RR-DQ1VI6Y
type: review-response
title: Create's early checkOrgID looks redundant
finding: derive now runs the same check, so the call in Provisioner.Create may be deleted as dead code.
severity: nit
resolution: A comment says Create checks first so that create reports the reason rather than an unknown organization.
status: addressed
---
