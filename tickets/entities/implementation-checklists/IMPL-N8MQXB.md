---
id: IMPL-N8MQXB
type: implementation-checklist
title: 'Implementation: Realtime 2/3: agents claim tickets through the rela CLI'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (ttl parsing, stdin lease id, lease.UserText mapping)
- [x] Integration tests written (test full flow, not just units) (two real subprocess claims; a second Services instance sees claim and release through its Watch)
- [x] Happy path implemented
- [x] Edge cases from planning handled (absent vs empty status, stdin terminal and empty input, unset USER, memory backend)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (runCLI helper over a temp project)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the subprocess test runs the real binary path end to end)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (expired lease written to the on-disk store becomes claimable)

**Verification Evidence:** AC1: two subprocess claims, one wins, the loser's
lease held names the winner's label. AC2: an expired lease from an abandoned
process is claimable. AC3: a second instance sees the claim within 2 s and the
release within 2 s (the HTTP _lease attach is covered in internal/dataentry).
AC4: renew lost text; release succeeds for unknown, expired and foreign ids.
AC5: list output and captured logs carry no lease id. AC6: texts come from
lease.UserText shared with MCP; not-found uses the rela show path. AC7:
CLAUDE.md claim loop and cli-reference updated.

## Quality

- [x] Code follows project patterns (check similar code) (kong command group, output formatter)
- [x] Checked for DRY opportunities (lease.UserText shared by CLI and MCP)
- [x] No security issues introduced (security review: two minor and one nit, fixed)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
