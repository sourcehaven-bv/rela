---
id: IMPL-FSFSO5P
type: implementation-checklist
title: 'Implementation: Fixed helper commands inherit REMOTE_CREDENTIAL_ variables'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestExecWithoutEnv covers the matcher)
- [x] Integration tests written (test full flow, not just units) (TestExecCommandsSetEnv scans the whole tree)
- [x] Happy path implemented
- [x] Edge cases from planning handled (two calls in one function, other packages' Command)
- [x] Error handling in place (errors surfaced, not swallowed) (gitConfig returns the error; callers keep the old fallback)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: tests parse inline Go source)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: assertions compare finding counts)
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolated values)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property comparisons)

## Manual Verification

- [x] Feature manually tested end-to-end (removed one Env line; the guard failed naming the file and line)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Deleting `cmd.Env = cmdexec.Environ()` from
docscapture/server.go made TestExecCommandsSetEnv fail at line 324; restoring it
made it pass. Tests for archguard, automation, cmdexec and docscapture pass;
GOOS=linux go vet passes for the bwrap probe.

## Quality

- [x] Code follows project patterns (check similar code) (guard uses the shared archguard allowlist helpers)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (the two git config calls share gitConfig)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
