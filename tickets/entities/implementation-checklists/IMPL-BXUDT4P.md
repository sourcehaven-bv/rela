---
id: IMPL-BXUDT4P
type: implementation-checklist
title: 'Implementation: Configurable dashboard validation card and entity header History and Add to pile'
started: "2026-11-19"
completed: "2026-11-19"
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
- Go: TestChromeToggle_*, TestValidateConfig_PageChrome, TestValidateEntityDashboards (dataentryconfig); TestPageChrome_* (dataentry handlers, per principal, no acl.yaml); TestDataEntryPermissions_* (cli audit). All pass.
- vitest: DashboardView.test.ts (43 pass), EntityDetail.world.test.ts (83 pass), both blocks of the header checked.
- Playwright: e2e/tests/page-chrome.spec.ts, 4 pass against the built rela-server: `validation: false` on a named dashboard; gated Add to pile shown without acl.yaml, shown for a holder under acl.yaml, absent for a principal without the permission.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
