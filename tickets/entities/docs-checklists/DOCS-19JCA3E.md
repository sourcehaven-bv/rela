---
id: DOCS-19JCA3E
type: docs-checklist
title: 'Docs: TKT-59HWQ76'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious: why the read-only write exceptions require a clean unescaped path, why the usage audit query is sent only when the table exists, why an operator principal is refused on tenant paths, why the control DDL has a lock timeout
- [x] Function/type docs if public API: godoc on `tenant.Change`, `Entitlements`, `Usage`, `Record.Archived`, `Provisioner.Apply`/`Archive`, `tenanthost.TenantControl`, `TenantAPIConfig`, `ReadOnly`; comment-lint `doclink` gate clean

## Project Documentation

- [x] ~~README updated (if applicable)~~ (N/A: no project-level change)
- [x] ~~CLAUDE.md updated (if new patterns)~~ (N/A: no new cross-cutting convention)
- [x] Help text accurate (if CLI changes): `-tenant-api-role` flag and `RELA_TENANT_API_ROLE` env replace `-tenant-api-audience`; `tenant show` prints read-only, plan and the api/archive causes

## User-facing Documentation

- [x] `GUIDE-postgres-backend.md` and generated `docs/postgres-backend.md`: section "Managing tenants through the tenant API" (edge operator assertion, role, states, PATCH rules, read-only scope and its gaps, archiving, usage, creating) and the webhook-mode row naming `-tenant-api-role`
- [x] ~~Security guide~~ (N/A: the trust model is described in the tenant API section; no ACL policy change)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the release changelog is generated from commits)
- [x] ~~API docs updated (if applicable)~~ (N/A: the tenant API is an operator endpoint outside the OpenAPI data-entry spec; documented in the guide)
