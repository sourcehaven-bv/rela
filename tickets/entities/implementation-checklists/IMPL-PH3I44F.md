---
id: IMPL-PH3I44F
type: implementation-checklist
title: 'Implementation: Flows M1e: flow worker'
started: "2026-11-11"
completed: "2026-11-11"
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

**Verification Evidence:** A scratch fs project in `.ignored/e2e` with a flow
whose state has a 1m timer, run with locally built `rela` and `rela-server`. A
run started while no server ran was escalated on the server's first tick ("flow
worker: due runs advanced=1"). A run started while the server ran was escalated
68s after its start. A second rela-server on the same project never logged "this
process leads". Both shut down cleanly. Automated: `TestFlowWorker_*` in
`internal/appbuild` pass on the default (fs), `sqlite` and `postgres` builds
(criteria 1 to 4 and 6); `TestFlowWorker_TickCap`, `NextTickResumes`,
`OnlyTheLeaderRuns` and `StopsLeadingWhenTheLockIsLost` (criterion 5 and the
leader edge cases); `flowengine` due tests for the round cap and cursor, the
join poll, a failed rejoin and a cut-off create; `TestCheckState_ForkedShape`;
`TestTryKeyedLock_CountsHeldLocks`. The vitest suites for the flows components
and i18n pass, and `npm run typecheck` is clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
