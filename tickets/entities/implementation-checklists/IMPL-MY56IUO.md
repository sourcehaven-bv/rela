---
id: IMPL-MY56IUO
type: implementation-checklist
title: 'Implementation: rela-server: multi-tenant host that routes a verified org_id to its tenant''s App'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (tenanthost host/memo tests, registry single-flight/ceiling/closed tests, config password tests, StaticShell test, validateTenantMode table)
- [x] Integration tests written (test full flow, not just units) (cmd/rela-server TestTenantMode_TenantsCannotSeeEachOther and TestTenantMode_FailsClosedOverRealTenants on postgres; tenant isolation suite incl. audit and missing-schema)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no/invalid assertion, no org, unknown org, open failure, ceiling, dot-segment paths, webhook org routing)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (fake registry/verifier, counterOpener, backendtest.SchemaDSN)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (via the postgres httptest suite over two real tenant schemas)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Two-tenant postgres test: an entity created in org-a
is not visible to org-b via GET, list or _search, and vice versa. Every
fail-closed path returns its status and opens no tenant. Per-tenant audit files
contain only their own record. A tenant whose DSN lands in another schema is
refused before migration.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (tenantsFile reuses tenant.ConfigFileName; appIdentity shared by single- and multi-tenant builds)
- [x] No security issues introduced (security review findings addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
