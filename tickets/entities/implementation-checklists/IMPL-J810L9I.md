---
id: IMPL-J810L9I
type: implementation-checklist
title: 'Implementation: Flows M1c: _flow-run type, relations, ACL and provenance'
started: "2026-11-07"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: WithFlowRuns is not wired into a running server until TKT-9IT3LN8; the appbuild test drives the production metamodel adapter instead)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: no running surface yet; edge cases are covered by tests)

**Verification Evidence:**

- AC1 reserved prefix: `internal/metamodel/flowrun_test.go`, `validateManagedReferences` tests.
- AC3 only system:flow writes: `TestFlowRunWrite_OnlyEngine` (acl), `TestManagedTypeGuard` and `TestManagedWriteDenial` (entitymanager), `TestFlowRunACL_ThroughMetamodelView` (appbuild).
- AC4 visibility follows `_carries`: `TestFlowRunReadQuery`, row gate per id subtest, storetest `Any_HasOutbound_branches` on mem, fs, sqlite and pg.
- AC5 provenance: `TestFlowProvenance`, `TestRecordSource`, `TestStamp`.
- #2106 regression: `TestValidateAgainstMetamodel_VisibleAliasKey`.
- `go test -race` passes on acl, audit, entitymanager, metamodel, principal, automation, importer, datamigration, validation, dataentry, cli, appbuild, visibility, fsimport, scheduler, lua, archguard and mcp.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
