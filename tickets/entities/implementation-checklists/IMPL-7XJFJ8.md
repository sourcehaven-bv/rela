---
id: IMPL-7XJFJ8
type: implementation-checklist
title: 'Implementation: rela channel: run message actions and show session state'
started: "2026-10-29"
completed: "2026-10-29"
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
- Tests: TestChannelCLI_Action (promote as member, text and JSON; lead-only close by system:ci; already run, undeclared, role without it, missing seq, release without pending, seq 0, name+release, neither) and TestChannelCLI_Session (text, JSON, after a move, no_session, hidden anchor). Race run of internal/cli passes.
- Manual, built bin/rela on a copy of the rela-mission project: `channel session TKT-001 run` printed review / waiting on you / may post: accept, rework. `channel action TKT-001 run 23 record` created DEC-001; a second run answered action_taken; `nope` answered action_not_offered; `--release` answered action_not_pending.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
