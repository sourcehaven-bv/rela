---
id: PLAN-0DHUD4
type: planning-checklist
title: 'Planning: jobs.Queue seam over neoq: ephemeral memory backend for FS/desktop, durable postgres for networked; migrate scheduler onto it'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: `internal/jobs` seam (flat retry enum + optional deadline), `jobstest`
conformance harness, memory backend (default build) + postgres backend
(`postgres` tag), commit-deferred enqueue, scheduler migration onto the seam.

OUT: transactional enqueue; moving `internal/lua/http.go` and `internal/ai` off
the inline path (Lua API semantic change — separate ticket); job-monitoring UI;
upstreaming `WithPgxPool` (post-working-system).

**Acceptance Criteria:**

1. `jobs.Queue` exists with a flat `Retry` enum and optional deadline; no
attempt counts / backoff curves are expressible at a call site. *Test:*
compile-time — the `Enqueue` signature admits no tuning params; `jobstest`
asserts each enum value's observable behaviour, not its numbers.
2. A job enqueued inside `store.Store.Tx` does not become runnable until the tx
commits. *Test:* `jobstest` run-after-commit test (deterministic, below).
3. A job enqueued outside a tx runs promptly. *Test:* `jobstest` baseline.
4. `RetryNever` runs exactly once on failure. *Test:* handler counts attempts.
5. `RetryBounded` retries then gives up. *Test:* attempts > 1 and terminates.
6. A job past its deadline is not run. *Test:* enqueue with past deadline.
7. Default build links no pgx; postgres build links no bleve.
*Test:* existing `ci.yml:495-498` greps, extended to the jobs packages.
8. Scheduler behaviour preserved: last-*successful*-run semantics, clock-jump
guard, `run_as` attribution, non-overlap. *Test:* existing `scheduler_test.go`
suite must stay green.

## Research

- [x] For larger features: run `/research` — N/A, research done inline in the
ticket (library survey with empirical dependency verification)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — survey recorded in TKT-YOED3R.

**Existing Solutions:**

- **neoq v0.72.1** (chosen) — backend-agnostic by design; same handler code on
memory or postgres. Empirically verified: a program importing only
`backends/memory` links **0** pgx / redis / asynq packages (35 deps total).
`jobs.Job` already carries `Deadline`, `RunAfter`, `MaxRetries`, and
`Fingerprint` (md5 of queue+payload — native dedup), which maps onto the seam
with no adapter gymnastics. Upstream is `code.adriano.fyi` (GitHub is a mirror);
v0.72.1 is current on both.
- **River** (rejected) — MPL-2.0, 5.6k★, extremely active. Core also links 0 pgx.
Rejected: transactional enqueue is not a requirement, its split is
Postgres/SQLite rather than memory/Postgres, and durable periodic jobs are a
paid River Pro feature.
- **asynq** (rejected) — Redis-only; adds a service rela does not run.
- **Prior art in-tree (reused, not reinvented):**
  - `pgstore.txPending` — the commit-deferral mechanism (`pgstore.go:250`,
`:268`; `tx.go:88-91`). Store events already guarantee "subscriber never
observes uncommitted state"; job enqueues join that list.
  - `storetest` / `statetest` — the conformance-harness pattern `jobstest`
follows.
  - `state.KV` — precedent for a small interface with per-backend impls.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`internal/jobs` defines the seam — no neoq types in the signature:

```go
type Retry int
const (
    RetryNever Retry = iota
    RetryBounded
    RetryPersistent
)

type Job struct {
    Kind     string
    Payload  map[string]any
    Retry    Retry
    Deadline time.Time // zero = none
}

type Queue interface {
    Enqueue(ctx context.Context, job Job) error
    Register(kind string, h Handler) error
    Start(ctx context.Context) error
    Close(ctx context.Context) error
}
```

Retry *mechanism* lives in one unexported table mapping `Retry` → neoq
`MaxRetries` (+ the `RetryPersistent` outer time bound). Changing tuning means
editing that table only — the reason for the enum's shape.

**Commit deferral** is the subtle part. `jobs.Queue` must not import `store`
(arch-lint: no cycle), so the deferral is a *decorator* in the jobs package
driven by a ctx marker the store sets. Concretely: a `deferred` wrapper holds
enqueues in a ctx-carried collector; `Tx` flushes it after commit. The collector
is populated only when a tx is open, so the non-tx path stays a direct enqueue.

**Files:**
- NEW `internal/jobs/jobs.go` — seam, `Retry`, `Job`, `Queue`, `Handler`
- NEW `internal/jobs/retry.go` — enum → mechanism table (the one tuning site)
- NEW `internal/jobs/deferred.go` — commit-deferral decorator + ctx collector
- NEW `internal/jobs/memqueue.go` — neoq memory backend (no build tag)
- NEW `internal/jobs/pgqueue.go` — neoq postgres backend (`//go:build postgres`)
- NEW `internal/jobs/jobstest/jobstest.go` — conformance harness
- MOD `internal/scheduler/scheduler.go` — execute via the queue
- MOD `internal/appbuild/appbuild.go` + `appbuild_{fs,memory,postgres}.go` — wiring
- MOD `.go-arch-lint.yml` — new `jobs` component
- MOD `.testcoverage.yml` — floor for the new package

**Alternatives rejected:** hand-rolled queue (re-solves backoff/dedup/durability
badly); putting the deferral in `store` (drags job concepts into the store);
generic `Queue[T]` (adds type ceremony for one payload shape).

## Security Considerations

- [x] Input sources identified
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- **Job payloads** — produced in-process, not user-supplied over the wire.
Serialized as JSON. Handlers must not trust payload contents as authority: a
payload names *what* to act on, never *what is permitted*.
- **`schedules.yaml`** — operator-authored config, already parsed and validated
by `scheduler.ParseConfig`. Unchanged by this work.
- **DSN** — stays `RELA_DATABASE_URL`-only (CLAUDE.md); never a flag, never in a
job payload.

**Security-Sensitive Operations:**

- **Principal attribution.** A job MUST carry the enqueueing principal and
re-stamp it on the handler ctx. Reads inside a handler are ACL-bound to that
principal (DEC-O59WM4). Never infer allow-all from "it's a background job" — the
scheduler's existing `stampTaskAuditContext` / `run_as` path is preserved
verbatim, not reimplemented.
- **Audit.** `audit.WithTriggeredBy` labelling must survive the hop through the
queue, or writes made by a job become unattributable.
- **Error logging.** Job errors are logged with kind + attempt count; payloads
are NOT logged wholesale, since they may carry entity content.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

`jobstest.RunAll(t, newQueue)` — every backend must pass, following
`storetest`/`statetest`. Covers AC 1-6:

- *Enqueue/run round trip* (AC3) — handler observes the payload.
- **Run-after-commit** (AC2) — the load-bearing one, made deterministic:
enqueue inside `Tx`; the handler signals on a channel; the test asserts **no
signal arrives before the tx commits**, then asserts it arrives after. Failure
mode is a *received-too-early*, not a flake.
- *Retry semantics* (AC4/5) — an always-failing handler counts attempts:
`RetryNever` ⇒ exactly 1; `RetryBounded` ⇒ >1 and terminates. Asserted as
behaviour, never as a specific number, so retuning the table doesn't break tests
(this is the enum's whole point).
- *Deadline* (AC6) — a past deadline never runs.

**Edge Cases:**

- Enqueue on a closed/not-started queue → error, not panic.
- Deadline already past at enqueue → dropped, not run-then-fail.
- Zero deadline → means "no deadline", not "epoch, expire immediately".
(Explicit test: the zero-value trap.)
- Handler panic → recovered, counted as failure (neoq `RecoveryCallback`).
- Rolled-back tx → deferred enqueue never fires.
- Nested `Tx` (joins the outer tx) → flush once, at the OUTER commit.
- Concurrent enqueue from multiple goroutines → no lost jobs.
- Process exit with queued jobs → memory backend drops them (asserted as
*intended* behaviour so nobody later "fixes" it).

**Negative Tests:**

- `Register` with a duplicate kind → error.
- `Enqueue` with an unregistered kind → error at enqueue, not silent drop.
- Empty `Kind` → rejected.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated: **L**

**Risks:**

1. **Scheduler regression (highest).** Its retry ladder encodes hard-won
fixes (BUG-ZKK2UL clock-jump guard; last-*successful*-run semantics that
RR-F6182G/RR-3BCWQ4 caught a revert of). *Mitigation:* keep `schedules.yaml`,
`State`, and the due-logic intact; the queue replaces *execution*, not
scheduling bookkeeping. The existing 939-line `scheduler_test.go` must stay
green untouched — if it needs editing to pass, that is a behaviour change and a
signal to stop.
2. **Non-overlap silently lost.** Today's scheduler is single-threaded, so
non-overlap is free; a concurrent worker pool would lose it invisibly.
*Mitigation:* pin concurrency to 1 per task queue and assert overlap never
happens in a test.
3. **pgx skew** (neoq pins 5.6.0, rela on 5.10.0). Accepted per ticket
Decisions; caught by our own tests.
4. **Second connection pool** on the postgres build. Accepted for now;
`WithPgxPool` upstreamed later.
5. **Arch-lint / dependency isolation.** A stray import could link pgx into the
default build. *Mitigation:* CI greps already exist; keep the pg backend in a
build-tagged file.

## Documentation Planning

- [x] User-facing docs identified
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] CLAUDE.md — new `internal/jobs` package + the "jobs run after commit" and
"retry enum stays vague" rules belong in the architecture rules
- [x] docs/ — a background-jobs page (deployment tiers + ephemeral-vs-durable)
- [x] ~~docs/cli-reference.md~~ (N/A: no new commands)
- [x] ~~docs/metamodel.md~~ (N/A: no schema change)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Design was reviewed iteratively with the user during
ticket authoring; four decisions were recorded on TKT-YOED3R (run-after-commit
invariant, flat-enum retry, own pool for now, pgx skew accepted). Two
user-driven corrections were folded in before implementation: (1) cadence must
not leak into the queue — the scheduler maps cadence onto the generic deadline
primitive instead; (2) the retry contract must stay a vague flat enum rather
than a tuned policy object. No open findings.
