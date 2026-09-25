---
id: IMPL-QTW8PU
type: implementation-checklist
title: 'Implementation: Scheduler stalls: durable jobs over 30s never complete and block their idempotency key'
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

- `go test -race ./...` passes on the default build.
- `-tags postgres` against a local PostgreSQL 18: `schedulerstate/...`,
`scheduler`, `appbuild` (including the scheduler e2e tests), `jobs` and the
pgstore migration tests pass.
- The shared `schedulerstatetest` conformance suite passes on both `kvstate` and
`pgschedstate`: one active run per task, stale-version refusal, compare-and-set
start, lease reap, once-only child settlement, prune.
- Scheduler tests cover: the tick never waits, skip while active, lost run
abandoned and retried, duplicate delivery runs once, enqueue failure fails the
run, two schedulers sharing one store create one run, failed child fails the
for_each run, retry skips delivered subjects.
- Lint gates: golangci-lint (default and postgres tags), arch-lint, plimsoll,
comment-lint and coverage-check are clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
