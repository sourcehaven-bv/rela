---
id: IMPL-966ACA
type: implementation-checklist
title: 'Implementation: History restore of a deleted entity fails when its status is past the entry state'
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

- sqlite build, scratch project: created T-001 (open); a create with
status=done was refused (illegal entry state); moved T-001 open, doing, done;
deleted it; `rela restore T-001 1` brought it back with status done.
- Regression tests fail with the old EnforceCreate call and pass with the fix.
- Unit: TestEnforceRestore (absent, entry, guard held, when: skipped, guard
denied, nil guard, undeclared value),
TestEnforceRestore_AnyEnteringEdgeSuffices.
- Manager: RestoresPastEntry (faceless), FacedRestorePastEntry (faced; create
refused), GuardedStateIs403, UnenterableStatusIs422.
- HTTP: DeletedFacePastEntryState (200), PastEntryStateStillFieldGated (403),
PastEntryStateNeedsCreate (403).
- Archguard: TestRecreateOnlyFromRestore, TestRecreateEntryPoints.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
