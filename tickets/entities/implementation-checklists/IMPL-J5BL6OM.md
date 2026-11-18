---
id: IMPL-J5BL6OM
type: implementation-checklist
title: 'Implementation: Record entity and relation audit rows inside the write transaction (fail closed)'
started: "2026-11-18"
completed: "2026-11-18"
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
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values in the new tests)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Verification:** TestAuditFailureRollsTheWriteBack runs the real appbuild
wiring on sqlite and on a local postgres (create and update refused by an
audit_log trigger, nothing stored). Full default and sqlite suites pass;
postgres-tagged entitymanager, appbuild, audit, pgstore suites pass. The
postgres dataentry TestGoldenParity piles goldens fail identically on the base
commit (pre-existing, unrelated).
