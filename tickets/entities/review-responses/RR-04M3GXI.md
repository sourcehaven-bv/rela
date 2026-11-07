---
id: RR-04M3GXI
type: review-response
title: Org handler seam does not state its context contract
finding: HandleOrgEvent receives the bare request context with no principal or ACL request or timeout; a future implementer may miss binding identity or checking org_id against the tenant registry.
severity: minor
resolution: Interface godoc now states that ctx has no principal or ACL request or deadline and that OrgID must be checked against the tenants this deployment serves.
status: addressed
---
