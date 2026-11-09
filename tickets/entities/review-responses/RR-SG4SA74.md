---
id: RR-SG4SA74
type: review-response
title: rela validate refused a tenant project without --tenant
finding: validate was gated, so config-only validation of a tenant project failed on the default build or without --tenant, although the config checks read no tenant data.
severity: significant
resolution: validate is tenant-free at the gate. The scheduled mail recipient step is skipped without a tenant; --check opens through openProject and fails closed. TestValidate_TenantModeWithoutTenant pins it.
status: addressed
---
