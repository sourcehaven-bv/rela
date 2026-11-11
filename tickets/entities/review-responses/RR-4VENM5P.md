---
id: RR-4VENM5P
type: review-response
title: db commands outside a project migrate the base DSN into public
finding: Outside a project there is no tenants.yaml to see, so resolveDSN used $RELA_DATABASE_URL, which in a tenant deployment is the base DSN. db migrate would create rela's tables in public, which every tenant search_path includes. Found by both reviewers.
severity: significant
resolution: 'Without a tenant map, resolveDSN calls tenant.CheckBareTarget: it refuses when the connection''s schema holds no schema_version table but another schema does. Single-store databases, migrated or empty, pass unchanged. TestDB_RefusesATenantDatabaseOutsideAProject pins both cases; tenantMapFor now swallows only ErrNoProject.'
status: addressed
---
