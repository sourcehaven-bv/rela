---
id: RR-7SB9IM5
type: review-response
title: Scheduler acceptance test derived org-a's schema from svcA itself
finding: svcAschema computed the audit directory from svcA's own DSN, so a miswired svcA would still pass.
severity: minor
resolution: The test uses the schemaA that writeTwoTenantProject returns and asserts svcA lands in it.
status: addressed
---
