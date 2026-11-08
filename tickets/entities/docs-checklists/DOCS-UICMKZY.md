---
id: DOCS-UICMKZY
type: docs-checklist
title: 'Docs: rela-server multi-tenant host (TKT-D6MF3SW)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Code Documentation

- [x] Comments where logic isn't obvious (registry single-flight, ceiling, background close; tenanthost fail-closed routing; DSN pinning and schema check)
- [x] Function/type docs if public API (tenanthost package doc, Host, ReuseAssertion, ForOrg, tenant.WithMaxOpen, ErrAtCapacity, ErrClosed)

## Project Documentation

- [x] ~~README updated~~ (N/A: tenant mode is documented in the postgres backend guide)
- [x] ~~CLAUDE.md updated~~ (N/A: no new cross-cutting pattern; the per-tenant isolation rules live in package docs and the guides)
- [x] ~~Help text accurate~~ (N/A: no new CLI flags; tenant mode is switched by tenants.yaml)

## External Documentation

- [x] ~~Changelog entry added~~ (N/A: the repo has no changelog; release notes come from PR titles)
- [x] API docs updated (GUIDE-postgres-backend "Tenant mode" section and GUIDE-acl-security org_id paragraph, regenerated into docs/)
