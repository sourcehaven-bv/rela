---
id: IMPL-FJAA2HA
type: implementation-checklist
title: 'Implementation: Flows M4b: job steps'
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

**Verification Evidence:**

- `internal/flowengine/job_test.go`: success, redelivery, exhausted
retries then operator retry with a fresh count, permanent error, `retry: false`,
timeout and lost worker (idempotent and not), a never-started attempt, a stale
redelivery, an enqueue error, no queue, the idempotency keys, a job in one fork
branch, and retry while running.
- `internal/appbuild/flow_jobs_test.go` runs the real wiring and queue:
a script job and an action job write as run_as; an action whose permission
run_as lacks fails permanently and does not run; a webhook posts ids only with
both keys and a valid signature to an httptest receiver; a 400 fails the job at
once; an unknown or `set` action stops the build.
- `internal/appbuild/flowjobs_internal_test.go`: status classification,
signature, payload round trip, no redirect, network error retries, missing
secret sends nothing.
- `FlowRunCard.test.ts`: the job line and its failed alert.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
