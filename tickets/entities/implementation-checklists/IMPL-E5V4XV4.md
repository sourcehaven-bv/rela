---
id: IMPL-E5V4XV4
type: implementation-checklist
title: 'Implementation: TestSSEACL tests flake on a fixed settle() sleep'
started: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (the change is the test harness; all 12 TestSSEACL tests use the new drain barrier)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: test-only change; the harness already drives the real runSSELoop and change feed)
- [x] Happy path implemented
- [x] Edge cases from planning handled (negative checks wait on the ping-token barrier; debounce tests use a 1s window; broker frames wait for their frames)
- [x] Error handling in place (errors surfaced, not swallowed) (drain and pingSeq fail the test with the sink contents)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: existing newTestAppV1 and seed helpers are unchanged)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions unchanged)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: test-only change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (mutation: forcing typeVisible to true fails all six negative checks)

**Verification Evidence:**
`go test -race -count=50 -run TestSSEACL ./internal/dataentry/` passed (190s).
A leaking gate (typeVisible always true) fails PerTypeGating,
RoleRelationInheritance, DenyAllWithholds, TwoPrincipalsDifferentFrames,
MembershipChangeReGates and FailClosedOnZeroVerdict.

## Quality

- [x] Code follows project patterns (check similar code) (signal-based waits as in sse_resume_test.go and live_served_test.go)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (test-only)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
