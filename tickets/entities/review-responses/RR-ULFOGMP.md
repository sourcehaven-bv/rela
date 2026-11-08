---
id: RR-ULFOGMP
type: review-response
title: 'Small cleanups: unused EnvTenant, error text, map read twice, validate opens twice'
finding: EnvTenant was unused; ErrTenantRequired did not mention $RELA_TENANT; the gate and openProject each read tenants.yaml; validate opens the tenant twice.
severity: nit
resolution: EnvTenant removed and the error mentions $RELA_TENANT. The double map read is cheap and both reads fail closed; validate's two opens predate this change. Both left as they are.
status: addressed
---
