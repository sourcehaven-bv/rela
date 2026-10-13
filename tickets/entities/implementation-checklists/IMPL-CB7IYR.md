---
id: IMPL-CB7IYR
type: implementation-checklist
title: 'Implementation: Realtime 3/1: realtime.yaml loader, validation and hot reload'
started: "2026-10-12"
status: done
completed: "2026-10-12"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: no runtime consumer until T3; boot, reload and config-error frame covered by appbuild and dataentry integration tests)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases pinned by fixtures in TestParse_Invalid)

**Verification Evidence:**
Tests on the T1 packages pass with -race (realtimecfg 96.4%, realtimelimits 100%).
- AC1/AC3/AC4: TestParse_Golden, TestParse_Invalid (each fixture names file, key path, reason), TestRealtime_InjectedACLStillChecksVisibleGrants.
- AC2: TestParse_Invalid (unknown keys, session: not supported yet).
- AC5/AC6: TestRealtime_Reload, TestRealtime_SchemaReloadFallback, TestCheckReload, TestReloadRealtime_Broadcasts (config-error frame).
- AC7: TestCheckEdit_RealtimeLocked, TestSave_RefusesRealtime.
- AC8: TestPermissionConsumerFor_IncludesRealtime, TestScanPermissions.
- AC9: just arch-lint rule metamodel -/-> realtimecfg.
- AC10: TestPeriod and realtimelimits AtMost tests (count and average).
- Boot: TestRealtime_AbsentFileIsOff, TestRealtime_InvalidFileFailsBoot.
- Review fixes: TestSource_NeedsCeilingCheck, TestCompiled_AccessorsReturnCopies, TestAnchorMismatches, TestParse_BillionLaughsIsBounded.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
