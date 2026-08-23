---
id: background-jobs
type: concept
title: Background Jobs
description: Async execution of external-system side effects (mail, HTTP, AI) off the request/write path, with retries tiered by deployment model
package: internal/jobs
layer: infra
status: draft
---

## Description

Rela increasingly performs side effects against external systems — outbound
mail, remote HTTP APIs, LLM calls. These are slow and intermittently failing, so
running them inline on a user-facing write path (or the scheduler's single
goroutine) makes latency unbounded and failure handling ad hoc.

The background-jobs concept is a single `jobs.Queue` seam, consumer-side and
narrow, that every subsystem enqueues through. Durability is tiered to match
rela's deployment models rather than forcing one guarantee everywhere:

- **FS / desktop** — in-process, ephemeral. Jobs vanish on exit. This is the
*correct* semantic for a single-user local app, not a degraded one: an unsent
mail from a session that ended is not worth persisting.
- **PostgreSQL / networked** — durable, survives restart, safe across multiple
server processes.

## A job never runs before the transaction that scheduled it closes

**Invariant: a job enqueued inside an open `store.Store.Tx` must not become
runnable until that transaction commits.**

Without this, a worker picks the job up on a *different* connection and reads a
snapshot that cannot yet see the enqueueing transaction's writes. The handler
observes the pre-write world — a mail about an entity that "does not exist," a
notification carrying a stale property. It is a race, so it passes tests and
fails under load, which is what makes it worth an invariant rather than caller
discipline.

This is NOT the same as transactional enqueue (below). Transactional enqueue is
about a job that should never have *existed* because the tx rolled back. This is
about a job that exists correctly but *ran too early*. The second is a live bug
on any backend; the first is a rare annoyance.

The mechanism already exists in-tree and should be reused rather than
reinvented: `pgstore` defers post-commit side effects via `txPending`, so store
events are delivered only after `tx.Commit` (`pgstore.go:250`, `tx.go:88-91`).
Job enqueues belong in that same deferral. On fs/mem — where `Tx` is a write
mutex with no rollback — the same "hold until fn returns" ordering applies.

## The retry contract is intentionally vague

A job declares its retry appetite as a **flat enum** — not a tuned policy
object:

- `RetryNever` — one attempt; a failure is final
- `RetryBounded` — "try a few times, then give up"
- `RetryPersistent` — "keep trying; this should eventually get through"

**The enum names intent, not mechanism.** Attempt counts, backoff curves,
jitter, and the outer time bound are all the framework's to choose, and to
change later without touching a single call site. `RetryPersistent` does not
mean literally forever — a sane implementation might retry for ~48h and then
drop the job with an error log, because a job that has failed for two days needs
a human, not another attempt.

This vagueness is the point. A precise contract (`bounded(5, exponential(2s))`)
pushes tuning into every producer and makes retry behavior a codebase-wide sweep
to change. A vague one keeps tuning in one place. Resist requests to widen this
enum into a policy struct; if a call site truly needs different mechanics, that
is evidence for a new *intent* value, not for exposing knobs.

## The queue is scheduling-agnostic

Beyond the retry enum, a job may carry an optional **deadline** — "stop trying
after time T." That is the whole remaining policy surface.

The queue does not know what a schedule or a cadence is, and must not learn:
most job producers have no cadence at all, and teaching the queue about one
would couple all of them to a concept only the scheduler has. Callers that *do*
have a cadence express it through the deadline — the scheduler passes a task's
own next scheduled run, so a short-cadence task stops retrying right before its
next run re-submits it, while a daily task gets a real backoff ladder inside its
24h window. One primitive, both behaviors, no cadence knowledge in the queue.

## Not in scope: transactional enqueue

Job insert is not atomic with the entity write. Redis- and AMQP-backed queues do
not offer this either; rela's job payloads are notifications and API calls where
a job orphaned by a rolled-back write is a rare annoyance handled by an
idempotency guard in the handler, not a correctness hazard.

Note this is a *weaker* promise than the run-after-commit invariant above, and
the two are independent: deferring the enqueue to commit-time actually satisfies
much of what transactional enqueue would give, since a rolled-back tx never
reaches the deferred-enqueue step.
