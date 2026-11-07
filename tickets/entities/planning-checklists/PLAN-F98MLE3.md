---
id: PLAN-F98MLE3
type: planning-checklist
title: 'Planning: neoq: stop the cron goroutine on queue shutdown before tenant eviction'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the neoq fork fix (sourcehaven-bv/neoq#3, merged as 7cfa988), the
go.mod replace bump, the go.mod patch list, the Close doc in neoqqueue.go, the
jobs rule, and goroutine baseline tests for the memory and postgres queues. Out:
proposing the fix upstream (separate follow-up), tenant eviction itself.

**Acceptance Criteria:**
1. Opening, starting and closing a postgres queue 20 times through a schema-pinned DSN leaves the goroutine count at its baseline: `TestPostgresQueue_CloseReleasesGoroutines`.
2. The same holds for the memory queue, and the test fails on the old pin: `TestMemoryQueue_CloseReleasesGoroutines`.
3. `TestPostgresQueue_SchemaPinnedDSN` and `TestPostgresQueue_SurvivesAnotherProcessClosing` still pass.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small dependency fix)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: fix is a missing Stop call)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- robfig/cron v1 `Stop()` blocks until the scheduler goroutine exits; the postgres backend already called it, the memory backend did not.
- asynq `Run` waits for OS signals; `Start` plus `Shutdown` is the library's own non-blocking pair.
- No upstream issue or PR at acaloiaro/neoq covers this.
- rela uses goleak elsewhere, but a count against a baseline is used here because other tests in the package leave goroutines that goleak would flag.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Fix Shutdown in the fork (memory: `cron.Stop`; redis:
`Start` instead of `Run`, stop both in Shutdown). Bump the replace. Add a shared
test helper that runs a warm-up cycle, takes a baseline, runs 20 cycles and
polls up to 5 s for the count to return. Alternative rejected: working around it
in rela. neoq exposes no handle to the cron scheduler.

**Files to modify:** go.mod, go.sum, internal/jobs/neoqqueue.go,
internal/jobs/goroutines_test.go (new), internal/jobs/pgqueue_test.go,
.claude/rules/jobs.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new input)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None new. The DSN is unchanged operator config.

**Security-Sensitive Operations:** None. A goroutine leak is a slow resource
exhaustion under tenant churn; this removes it.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria. The postgres test runs against a
real database through a schema-pinned DSN.

**Edge Cases:**
- Goroutines still exiting when Close returns: the helper polls up to 5 s.
- Process-wide goroutines started on first use: excluded by the warm-up cycle.
- Close twice: covered by `TestPostgresQueue_CloseLifecycle`.

**Negative Tests:** The memory test fails on the old pin (20 goroutines over
baseline), confirmed before the bump.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Flaky goroutine counts: mitigated by the warm-up cycle, an at-most-baseline check and polling.
- Fork change merged without review because `sourcehaven` is unprotected: the rela PR asks the reviewer to look at it.
- Redis backend no longer shuts down on SIGTERM: rela does not use the redis backend.
Effort: s.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal change)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore, not an enhancement)

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed (the jobs rule and go.mod comment are updated in the change)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one-line dependency fix; approach agreed in the ticket)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None
