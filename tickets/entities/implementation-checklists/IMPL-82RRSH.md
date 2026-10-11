---
id: IMPL-82RRSH
type: implementation-checklist
title: 'Implementation: Access log records the route shape, not ids or file names'
started: "2026-10-09"
completed: "2026-10-09"
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

- [x] Feature manually tested end-to-end (TestRequestStats_AccessLogMasksIdentity drives the real router with --access-log wired)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: each edge case is a TestRouteShape row)

**Verification Evidence:** TestRouteShape, TestRouteShape_StopsAtCap,
TestFixedRouteWords_CoverRegisteredRoutes, TestRouteWordCache_FollowsSnapshot
and TestRequestStats_AccessLogMasksIdentity pass; go test ./internal/dataentry
passes; golangci-lint 0 issues.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
