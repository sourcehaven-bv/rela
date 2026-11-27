---
id: IMPL-5PXE1XB
type: implementation-checklist
title: 'Implementation: TestOutputs_TriggerFinalCloseOnly misses the close when a mark arrives mid-round'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestWorker_OutputsWakeDuringRoundReruns, TestWorker_OutputsNoLostWake)
- [x] Integration tests written (TestOutputs_TriggerFinalCloseOnly now forces the mid-round wake)
- [x] Happy path implemented
- [x] Edge cases from planning handled (wake in the release window, canceled lead, coalesced wakes)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (existing runAsServices env)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (integration test with a 3s sleep after each round: fails on develop, passes with the fix)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (mutation runs: dropping the rerun or the recheck fails the tests)

**Verification Evidence:**
<!-- Document what you tested and the results -->
New unit test fails on develop and passes with the fix. Slowed rounds reproduce
the CI failure on develop. go test -race -count=100 on TestOutputs_ and
TestWorker_Outputs passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
