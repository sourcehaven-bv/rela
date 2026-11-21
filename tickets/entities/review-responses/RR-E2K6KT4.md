---
id: RR-E2K6KT4
type: review-response
title: Org id from the path reaches the database
finding: 'Code review: the path org id is used for Acquire and the provisioner.'
severity: minor
resolution: 'Addressed: it must equal the verified org_id and pass tenant.SchemaFor before any lookup; an encoded slash makes a nested path and gets 404. TestTenantAPI_InvalidOrgNeverReachesProvisioner pins it.'
status: addressed
---
