---
id: DOCS-1JRFUY9
type: docs-checklist
title: 'Docs: TKT-84CD0PY'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the API audience must be its own, why the token is bound to the path org, why a running create reads as provisioning
- [x] Function/type docs if public API: godoc on `TenantsPath` (with an Authentication section), `TenantAPIVerifier`, `TenantAPIClaims` and `Config.TenantAPI`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: reuses the webhook verifier pattern)
- [x] Help text accurate (if CLI changes): `-tenant-api-audience` flag usage names the env var and the distinct-audience rule

## User-facing Documentation

- [x] `GUIDE-postgres-backend.md` and `docs/postgres-backend.md`: new "Creating tenants through the tenant API" section (requests, authentication, token claims, responses) and the `webhook` row of the provisioning table
- [x] Security guide: the authentication and trust assumption are in the same section, next to the provisioning trust assumption

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: a host-level route outside the tenant OpenAPI spec, like `/api/v1/_tenant`)
