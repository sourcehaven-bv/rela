---
id: IMPL-H0AUVJ8
type: implementation-checklist
title: 'Implementation: Run the full race suites after merge, not in the merge queue'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: CI workflow and shell script; exercised by CI itself)
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no Go tests added)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no Go tests added)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no Go tests added)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no Go tests added)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no Go tests added)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `scripts/ci-race-gate.sh` run locally: exit 0, 142 packages ok, 2m36s.
- `go test -race -v` with the conformance patterns shows
`TestConformance/CAS/ConcurrentAppendersAllLandWithRetry` and the `Concurrency`
subtests of kvlease, kvpiles and memcomments running.
- The no-match edge case fails through the explicit `::error::` branch
(`xargs -r`, empty `pkgs` check); shellcheck is clean.
- actionlint: no new findings against develop's ci.yml.
- The PR run on #2348 checks the PR path; the push path runs after merge.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
