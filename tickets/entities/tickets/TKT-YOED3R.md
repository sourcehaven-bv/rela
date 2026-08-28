---
id: TKT-YOED3R
type: ticket
title: 'jobs.Queue seam over neoq: ephemeral memory backend for FS/desktop, durable postgres for networked; migrate scheduler onto it'
kind: enhancement
priority: medium
effort: l
tags: needs-design
status: done
---

## Problem

External-system side effects (outbound mail, remote HTTP APIs, LLM calls) run
inline today. Two consequences:

1. **Unbounded latency on user-facing paths.** A slow remote endpoint blocks the
goroutine serving the user. `internal/lua/http.go` and `internal/ai` are
reachable from the Lua write path, so an automation can stall a write on a
third-party timeout.
2. **Ad-hoc retry logic.** `internal/scheduler` has a hand-rolled retry ladder
(5m→2h, `scheduler.go:50-62`) that *replaces* a task's schedule while it is
failing. It works, but it is one worker's private bookkeeping in
`.rela/scheduler-state.json`, not a reusable facility — and nothing else that
does external IO has an equivalent.

## Approach

Introduce a narrow consumer-side `jobs.Queue` seam (mirroring how `store.Store`
is depended on directly) with the backend selected per deployment tier:

| Tier | Backend | Guarantee |
|---|---|---|
| FS / desktop (default build) | neoq memory backend | Ephemeral — jobs vanish on exit, by design |
| PostgreSQL (`postgres` tag) | neoq postgres backend | Durable, survives restart, multi-process |

**Why neoq** ([code.adriano.fyi/me/neoq](https://code.adriano.fyi/me/neoq),
GitHub is a mirror): it is backend-agnostic by design — the same handler code
runs on memory or postgres — which matches the tiering above exactly. Verified:
a program importing only `backends/memory` links **0** pgx, redis, and asynq
packages (35 deps total), so it respects the build-tag isolation CI asserts in
`ci.yml:495-496` (default build must not link pgx).

Alternative considered: **River** (MPL-2.0, 5.6k★, very actively maintained).
Its core also links 0 pgx packages, and it offers transactional enqueue.
Rejected because transactional enqueue is not a requirement here (see the
[[background-jobs]] concept — Redis/AMQP queues do not provide it either), and
River's backend split is Postgres/SQLite rather than the memory/Postgres tiering
rela wants. River's "durable periodic jobs" is also a paid River Pro feature,
which is adjacent to the scheduler work in scope here.

## Invariant: a job never runs before its enqueueing Tx closes

**A job enqueued inside an open `store.Store.Tx` MUST NOT become runnable until
that transaction commits.**

The failure this prevents: a worker picks the job up on a *different* connection
and reads a snapshot that cannot see the enqueueing transaction's writes yet.
The handler acts on the pre-write world — mails about an entity that "does not
exist," notifications carrying stale properties. It is a race, so it passes
tests and fails under load. That is why it is an invariant of the seam rather
than caller discipline.

**Distinct from transactional enqueue** (which stays out of scope): that is
about a job that should never have *existed* because the tx rolled back; this is
about a job that exists correctly but *ran too early*. Deferring to commit-time
incidentally covers much of the rollback case too — a rolled-back tx never
reaches the deferred enqueue.

**Reuse the existing mechanism, do not invent one.** pgstore already defers
post-commit side effects through `txPending`: `emit`/`emitAll` check
`s.txPending != nil` and queue a note (`pgstore.go:250`, `:268`), which `Tx`
runs only after `tx.Commit` succeeds (`tx.go:88-91`). Store events already have
exactly the property we need — subscribers never observe uncommitted state. Job
enqueues belong in that same deferral list.

Backend notes:
- **pgstore** — enqueue becomes a `txPending` note; runs after commit, skipped on
rollback.
- **fsstore/memstore** — `Tx` is a write mutex with no rollback, so the ordering
requirement is just "hold the enqueue until fn returns." Same shape, weaker
underlying guarantee, consistent observable behaviour.

**Test this explicitly.** A race that only appears under load needs a
deterministic test: enqueue inside a `Tx`, have the handler assert it can read
the transaction's write, and gate the tx's commit so the test fails if the job
is runnable early. Belongs in `jobstest` so every backend must satisfy it.

## The retry contract: a flat enum, deliberately vague

Keep the seam as high-level as possible. A job declares its retry appetite as a
**flat enum**, never a tuned policy object:

```go
type Retry int

const (
    RetryNever      Retry = iota // one attempt; failure is final
    RetryBounded                 // try a few times, then give up
    RetryPersistent              // keep trying; this should get through
)
```

**The enum names intent; the framework owns mechanism.** Attempt counts, backoff
curves, jitter and the outer time bound are implementation-side and can be
retuned without touching a call site. `RetryPersistent` explicitly does NOT mean
literally forever — dropping the job after ~48h with an error log is a valid
reading, because a job failing for two days needs a human, not another attempt.

This vagueness is the design goal, not a gap to be filled in later. A precise
contract (`bounded(5, exponential(2s))`) scatters tuning across every producer
and makes a retry-behaviour change a codebase-wide sweep. Reviewers should
**reject** PRs that widen this into a policy struct or add per-call knobs; a
call site that genuinely needs different mechanics is evidence for a new
*intent* value, not for exposing parameters.

The only other policy input is an optional per-job **deadline** ("stop trying
after time T"). See below for why that one earns its place.

## The queue knows nothing about cadence

**The queue must not know that a task has a schedule.** Cadence is the
scheduler's private concern; leaking it into the queue would couple every future
job producer (mail, HTTP, AI — none of which have a cadence) to a concept only
one of them has.

The deadline is what makes the cadence case expressible without cadence
awareness. When submitting a task the scheduler computes its own next scheduled
run and passes it as the job deadline: a 60s-cadence task submits with a
deadline ~60s out, so retries stop when the next run is about to fire and the
next tick re-submits it; a daily task passes a deadline 24h out and gets a real
backoff ladder inside that window. Same primitive, opposite behaviour, queue
none the wiser.

Deduplication is a genuine queue concern: a slow task on a short cadence must
not pile up N concurrent copies. The current scheduler cannot express this at
all (it is single-threaded, which prevents pile-up by preventing concurrency — a
property the migration must not silently drop).

## Scheduler migration

`internal/scheduler` is effectively a single-worker job queue already. Migrate
it onto `jobs.Queue` while keeping `schedules.yaml` unchanged — config surface
is stable, execution engine is swapped underneath.

Carry across, do not discard:
- The clock-jump guard (`retryAt.Sub(now) > maxRetryDelay`) — hard-won, fixes a
state file that would otherwise wedge a task forever (BUG-ZKK2UL).
- `stampTaskAuditContext` principal/audit attribution (DEC-O59WM4) — jobs must
carry the task's principal, and `run_as` must keep working.
- Suppression of normal cadence while a retry is pending.
- Non-overlap: one instance of a given task at a time.

## Scope

**In scope**
- `internal/jobs` — the `jobs.Queue` seam + conformance harness (`jobstest`,
following `storetest`/`statetest` precedent). Flat retry enum + optional
deadline only; no cadence awareness, no per-call tuning knobs. Includes the
run-after-commit conformance test.
- Commit-time enqueue deferral, reusing pgstore's `txPending` mechanism
- Memory + postgres backend wiring via `appbuild` recipes (per-tier, per build tag)
- Scheduler migration onto the seam, incl. mapping cadence → job deadline

**Out of scope**
- Transactional enqueue (see concept — deliberate; distinct from the
run-after-commit invariant, which IS in scope)
- Moving `internal/lua/http.go` / `internal/ai` off the inline path. This is a
*semantic* change to the Lua API (an automation could no longer read an HTTP
result inline), and deserves its own ticket + decision.
- A job-monitoring UI
- Upstreaming `WithPgxPool` to neoq — see Decisions.

## Decisions

- **Jobs run after commit, always.** An enqueue from inside a `Tx` is deferred
until the transaction closes, so a handler can never read a connection that
cannot see the committed state. Implemented on the existing `txPending`
deferral, and pinned by a `jobstest` conformance test.
- **Retry config is a flat enum, and stays vague.** Intent at the call site,
mechanism in the framework. The concrete meanings (attempt counts, backoff, the
`RetryPersistent` time bound) are free to change during implementation and
afterwards — that freedom is the reason for the shape.
- **Own pool for now.** The postgres backend gets its own connection pool via
`WithConnectionString` rather than sharing rela's injected pgx pool. This
departs from the CLAUDE.md pool-injection rule, accepted deliberately to avoid
blocking on an upstream change. Prepare the `WithPgxPool` PR *after* the system
works end-to-end — the field (`pool *pgxpool.Pool`) and guard (`if p.pool ==
nil`, `postgres_backend.go:168`) already exist upstream, so it stays a ~20-line
change whenever we get to it. Revisit if the second pool causes connection-count
pressure in a real deployment.
- **pgx skew is not a blocker.** neoq pins pgx v5.6.0, rela is on v5.10.0; Go
resolves to 5.10.0. PostgreSQL's wire protocol and pgx's API are both stable in
practice, and our own tests will catch any obvious breakage. No pre-emptive
work.
