---
id: IMPL-M6T5EEP
type: implementation-checklist
title: 'Implementation: tenant: derive tenant location from org_id; remove tenants.yaml'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built rela-server-postgres and ran it against a fresh
local database: create (active, org_<hash> schema), show, second create refused,
suspend, show suspended, resume, erase -force (schema dropped, erasure
recorded), show "no tenant", resume of the erased org refused. A single-store
server on that database refused to start (rela_control.tenants present), and a
project with tenants.yaml refused to start. Automated: go test -race on tenant,
cli, rela-server, rela-docs, tenanthost, dataentry, default and postgres tags
(schema-pinned test DB).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
