---
id: IMPL-HTZCASC
type: implementation-checklist
title: 'Implementation: tenant: erase an org — DROP SCHEMA CASCADE, a documented retention SLA, and the audit-log residue'
started: "2026-11-08"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (erase mark, ErasureTarget, shared-schema refusal, bad input, CLI refusals, help, extra args)
- [x] Integration tests written (test full flow, not just units) (postgres tests on private databases: two-tenant erase, lock, lost lock, public tables, audit failures, CLI end to end)
- [x] Happy path implemented
- [x] Edge cases from planning handled (rerun, missing schema or directory, symlink, all tenants marked)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (backendtest.Database, SchemaIn, isolationOpener)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (TestEraseTenant_EndToEnd drives the command over a real database)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Erasing org-a drops its schema and audit
directory; org-b's schema, rows and audit file remain; an open tenant
refuses the erase; a lost lock leaves the open store failing; the command
prints no DSN and records the erasure.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (AuditRoot shared by server and command; checkSchemaName shared by validate and derivation)
- [x] No security issues introduced (security review findings addressed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
