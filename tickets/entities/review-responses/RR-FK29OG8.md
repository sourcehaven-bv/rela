---
id: RR-FK29OG8
type: review-response
title: The audit-log fallback in the usage query never works
finding: PostgreSQL resolves audit_log at parse time, so the CASE on to_regclass did not help and a schema without the table returned 503. A provisioning tenant also returned 503.
severity: minor
resolution: Usage checks to_regclass first and only then sends the audit query. A provisioning tenant returns ErrTenantProvisioning, answered 409 tenant_provisioning. Tests cover a dropped audit table and a provisioning tenant.
status: addressed
---
