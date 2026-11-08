---
id: RR-74ZE3TE
type: review-response
title: Tenant app does not check the request's org
finding: Each tenant's gate and webhook receiver accepted any verified org; isolation rested only on host routing.
severity: minor
resolution: 'openTenant wraps the tenant''s verifiers in tenanthost.ForOrg and WebhookForOrg, which refuse another org. Test: TestForOrg_RefusesOtherOrgs.'
status: addressed
---
