---
id: RR-BCN2I6U
type: review-response
title: Tenant can silently run in a shared schema
finding: PostgreSQL skips a missing search_path schema, so a tenant listed before CREATE SCHEMA, or an explicit dsn without search_path, lands in public.
severity: significant
resolution: 'SharedBaseOpener checks current_schema() equals the tenant schema before creating the audit sink or running migrations, and refuses otherwise. Test: TestTenantIsolation_MissingSchemaRefused.'
status: addressed
---
