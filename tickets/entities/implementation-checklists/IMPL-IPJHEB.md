---
id: IMPL-IPJHEB
type: implementation-checklist
title: 'Implementation: Make filesystem migration stale-break acquisition atomic'
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
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no generated expected strings)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- The CI failure in PR #1450 showed two winners and two stale-break warnings on
the same lock path, confirming the replacement gap was exercised.
- `go test -race ./internal/datamigration -run 'TestFSLock_(ConcurrentStaleBreakSingleWinner|ExclusiveAndReleases|BreaksStaleAndUnparseable|HonorsLivePid|StaleReleaseDoesNotRemoveNewHoldersFile)' -count=200`: PASS.
- `go test -race ./internal/datamigration`: PASS.
- `just lint`: PASS with zero issues; `just arch-lint`: PASS.
- Full `just ci`: PASS, including both race runs, coverage (78.2%), builds,
  frontend, and generated-doc freshness.
- Inspection verifies the `.break` marker is acquired before inspection and
removed by one defer only after successful publication or failure return.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
