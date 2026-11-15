---
id: RR-4LBG3R2
type: review-response
title: Adapter reported provisioned for non-active existing tenants
finding: The provisioning adapter counted ErrTenantExists as success even when the existing tenant was suspended, erasing or still provisioning, so the host reported a ready tenant that was not active.
severity: minor
resolution: 'Fixed in 13cd61e18: tenant.ExistsError carries the existing status and the adapter treats only an active tenant as provisioned. Pinned by TestProvision_ExistsButNotActive.'
status: addressed
---
