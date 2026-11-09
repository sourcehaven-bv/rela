---
id: IMPL-IWLTKQG
type: implementation-checklist
title: 'Implementation: tenant: scheduler, CLI and stdio MCP in a multi-tenant deployment'
started: "2026-11-08"
completed: "2026-11-08"
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

**Verification Evidence:**

- AC1: `TestTenantScheduler_WritesOnlyIntoItsTenant` (postgres) runs
`SchedulerCmd` for org-a against two real schemas. The note, the run state and
the audit records land in org-a only.
- AC2: `TestCheckTenantMode`, `TestCheckTenantMode_EveryCommandIsGated`,
`TestOpenProject_TenantModeFailsClosed`,
`TestNewMCPServices_TenantModeFailsClosed` and `TestTenantMCP_SeesOnlyItsTenant`
cover a missing, unknown and erased tenant and two-tenant isolation over stdio
MCP.
- AC3: `--tenant` takes an org id only; `TestSelect_ErrorsNameNoDSN` pins that
errors name no DSN.
- Single-tenant mode: `TestOpenProject_SingleTenantUnchanged`, plus the full
existing `internal/cli` suite on the default and postgres tags.
- Mutation checks: removing `tenant.Pin` from `resolveDSN` fails
`TestTenantDB_RefusesAMissingSchema`; closing MCP through the services instead
of the `tenant.Opened` fails `TestTenantMCP_CloseReleasesTheSchemaLock`.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
