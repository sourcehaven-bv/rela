---
id: IMPL-Q7VPH3
type: implementation-checklist
title: 'Implementation: Live views 3/3: MCP watch tool'
started: "2026-10-11"
completed: "2026-10-11"
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

Automated (`internal/mcp/tools_watch_test.go`, through `NewServer` and a real
client over the in-memory transport): AC1 baseline, AC2 blocks until change, AC3
hidden writes do not wake (with a visible-write control), AC4 timeout keeps
digests and baselines added targets, AC5 change between calls, AC6 invalid-input
table, AC7 per-identity limit (concurrent) and server limit, AC8 cancel releases
the subscription, AC9 access re-resolved each pass (a middleware attaches a
stale `acl.Request` per call, as remote MCP does). Edge cases: duplicate
targets, target argument errors, burst replays once per tick, panicking replay,
digest keyed per process and principal, foreign cursor reports changed, Rebind
failure is a tool error. Mutation checks: removing `Rebind` fails AC9; dropping
added targets from the cursor fails AC4.

Manual, stdio (`rela mcp` on a copy of `prototypes/data-entry/project`, driven
over JSON-RPC):
- baseline in 2 ms;
- a blocked watch did not stall `update_entity` on the same connection
(76 ms);
- watch returned `changed: [0]` 0.5 s after an MCP write and 0.5 s after an
on-disk edit;
- idle watch timed out at 1.0 s.

Remote MCP is not driven by hand (it needs a JWT setup). Its one difference, the
per-call `acl.Request`, is reproduced in AC9.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
