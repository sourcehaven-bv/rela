---
id: IMPL-YZQTERW
type: implementation-checklist
title: 'Implementation: Flow run hidden from role-relation reader when inherit_roles_through is set'
started: "2026-11-17"
completed: "2026-11-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestFlowRunReadQuery_RoleRelationWithInheritance)
- [x] Integration tests written (TestFlowControl_RoleRelationWithInheritance: run view, subject runs, tasks, complete)
- [x] Happy path implemented
- [x] Edge cases from planning handled (inherited-only subject stays hidden; who-can skips ancestor routes)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (newFlowFixture, stepProjectWith)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: FlowControl test drives the service the HTTP handlers call; the saas-dev repro was not rerun)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (ancestor route filter checked by disabling it: the test fails)

**Verification Evidence:** Both regression tests fail on develop 0cc979402 (run
not found / no runs) and pass with the fix. go test of acl, aclmap, visibility,
dataentry, cli and appbuild passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities (gateHop split into gateHop + foldHop, shared by the run gate)
- [x] No security issues introduced (the change only narrows a read query; inherited reads stay hidden)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
