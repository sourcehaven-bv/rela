---
id: IMPL-8U8W38Y
type: implementation-checklist
title: 'Implementation: Threads 1: read one thread and the top-level stream'
started: "2026-11-12"
completed: "2026-11-12"
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

**Verification Evidence:** End to end through each transport against a real
project: `rela channel read --thread 1` and `--top` (TestChannelCLI_Loop), HTTP
`?thread=1`, `?top=1`, `top=0` and the 400s (TestChannels_ThreadPages), MCP
read_channel thread/top (TestChannelTools_RoundTrip,
TestChannelTools_Arguments). Store behaviour on file, sqlite and postgres via
channelstest ThreadPages, ThreadRemoval and ThreadPageRefusals. Migrations:
TestMigrate_v36ToV37ChannelThreads and TestMigration0045_ChannelThreads, each
opened twice.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
