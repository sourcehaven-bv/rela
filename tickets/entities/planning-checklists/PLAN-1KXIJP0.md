---
id: PLAN-1KXIJP0
type: planning-checklist
title: 'Planning: Replace neoq with River on postgres and an in-house memory queue'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: replace neoq's memory backend with rela's own in-process queue;
replace the neoq postgres adapter with River; move pending neoq jobs into River
once and drop the neoq tables; remove the neoq dependency and fork `replace`.
Out: enqueueing inside the store transaction with River's `InsertTx`; changes to
the `jobs.Queue` interface or to any caller.

**Acceptance Criteria:**
1. `jobstest.RunAll` passes against the memory queue and the River queue.
2. Closing a memory queue leaves no goroutine (`goleak` tests).
3. The River queue initializes against a schema-pinned DSN, with its tables
in that schema.
4. A CLI process closing its queue does not stop another process's workers.
5. Pending `neoq_jobs` rows are moved into River on first start, and the
neoq tables are dropped.
6. No neoq import remains; `go.mod` has no neoq `replace`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the comparison was done in the session that filed the ticket and is in its description)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- River (riverqueue/river, MPL-2.0): pgx v5 native, `Config.Schema`, unique
jobs by state, job cleaner, panic recovery, per-job `MaxAttempts`. Chosen.
- Absorbing neoq: rejected; we would own a queue engine with four known
concurrency bugs already patched in our fork.
- Memory tier: River needs a database, so the ephemeral tier is a small
queue of our own (slice, condition variable, fixed workers).
- Reused: `jobs.Job.validate`, `deferEnqueue`, `Retry.maxAttempts`,
`Retry.effectiveDeadline`, the `jobstest` conformance suite.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** PR 1: `memQueue` implements `jobs.Queue` directly. Retry
backoff moves into `retry.go` (`retryDelay`) so both tiers share it. Idempotency
hashing moves to `jobs.go` (`idempotencyID`). The neoq adapter becomes
postgres-only.

PR 2: `riverQueue` with one River job kind carrying rela's kind, payload, retry
intent, effective deadline and idempotency key. Unique jobs by args over the
pending states only. `MaxAttempts` per job from the retry intent; deadline and
handler lookup checked in the worker. Retention via River's job cleaner (24h).
Migration and the one-time neoq import run in one transaction under an advisory
lock. The queue keeps its own small pool: appbuild deliberately cannot name pgx
types, and River does not hold a connection while a handler runs, so the
pool-floor reason for sharing is gone.

**Files to modify:** internal/jobs/{jobs,memqueue,retry,neoqqueue,pgqueue}.go
and tests, internal/jobs/jobstest/jobstest.go,
internal/appbuild/memrecipe_test.go, go.mod, go.sum,
docs-project/entities/guides/GUIDE-postgres-backend.md, .claude/rules/jobs.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Job payloads come from rela subsystems, not
users; `Job.validate` rejects an empty kind and an out-of-range retry. Stored
neoq rows are read once at migration; a row with no kind is dropped with a log
line.

**Security-Sensitive Operations:** The DSN carries credentials and must never
reach an error string. Payloads are never logged (they may carry entity
content). Queue tables stay in the tenant schema, so tenants never consume each
other's jobs.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. `TestMemoryQueue_Conformance`, `TestPostgresQueue_Conformance`.
2. `TestMemoryQueue_CloseReleasesGoroutines`,
`TestNewInMemory_CloseStopsEveryGoroutine` without the cron exemption.
3. `TestPostgresQueue_SchemaPinnedDSN`.
4. `TestPostgresQueue_SurvivesAnotherProcessClosing`.
5. A test that seeds `neoq_jobs` rows in a scoped schema and checks they run.
6. `go build` and `go mod why` show no neoq.

**Edge Cases:** Close before Start; Close timing out on a stuck handler; a keyed
job between attempts; a retry due after the deadline; a handler that panics; a
caller editing its payload map after Enqueue; concurrent first starts migrating.

**Negative Tests:** Unknown kind, empty kind, duplicate registration, enqueue
after close and before start all return their sentinel errors (conformance
suite).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Lost jobs at cutover: the import copies pending rows in the same
transaction that drops the neoq tables. Old processes must be stopped before the
upgrade; documented in the postgres guide.
- Two processes migrating at once: an advisory lock serializes them.
- Behavior drift between tiers: both run the same conformance suite.
Effort: l.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: chore ticket; the only doc change is the postgres guide's queue tables section)

**Documentation Impact:**
- [x] docs-project GUIDE-postgres-backend: queue tables and upgrade note
- [x] .claude/rules/jobs.md: neoq fork paragraph

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see the review-response entities linked to
TKT-TMXHBZW
