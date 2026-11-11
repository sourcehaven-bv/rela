---
id: RR-KK0FWWO
type: review-response
title: 'Design: upgraded deployment falls back to public'
finding: Without tenants.yaml detection an upgraded v26.11.2 deployment that does not set RELA_TENANT_MODE starts single-store on the base DSN and migrates public.
severity: critical
resolution: 'RefuseLegacyMap refuses any entry point while tenants.yaml exists; RefuseTenantDatabase refuses a single-store open on public or a tenant schema of a database with rela_control.tenants. Tests: TestRefuseTenantDeployment_*, TestSingleStore_RefusesATenantDatabase, TestOpenProject_RefusesLegacyMap.'
status: addressed
---
