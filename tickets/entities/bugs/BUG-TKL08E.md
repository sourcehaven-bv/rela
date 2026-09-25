---
id: BUG-TKL08E
type: bug
title: 'Scheduler stalls: durable jobs over 30s never complete and block their idempotency key'
description: Postgres-tier jobs that run longer than 30s are killed by neoq's default idle-in-transaction timeout; the row never completes; reruns forever and blocks re-enqueue; the scheduler's synchronous in-process wait then stalls every task.
priority: high
effort: l
why1: A durable job row stayed pending (status new) indefinitely. Its idempotency key therefore rejected every later enqueue of the task as already pending; and the scheduler's synchronous wait for that run's result timed out after 20 minutes.
why2: neoq never recorded the job as processed. Its handler runs inside the transaction holding the row lock; after 30 s idle PostgreSQL kills that session (SQLSTATE 25P03); so updateJob fails; the row keeps status new and retries 0; and the pending poll reruns it every ~60 s forever.
why3: rela never set postgres.WithTransactionTimeout; so neoq's 30 s default idle_in_transaction_session_timeout applies to every worker connection. That default is shorter than handlerTimeout (15 m) which rela chose for the same work.
why4: The scheduler assumes exactly one completion signal per run delivered to an in-process channel in the submitting process. Its correctness depends on the queue's delivery being exactly-once and same-process; neoq is at-least-once and cross-process; so any lost or duplicated completion (rerun; restart; other node; handler timeout) turns into a stall of the single sequential scheduler goroutine.
why5: Run lifecycle state lives only in process memory and in neoq's internal table. Neither is a queryable; durable record rela owns. The jobstest conformance suite only uses millisecond handlers; so no test ever exercised a job longer than a backend timeout; and the queue seam had no contract for how long a handler may run.
prevention: AM-durable-job-outlives-idle-tx pins a handler longer than the idle-tx timeout completing once on the postgres queue. AM-scheduler-run-state-conformance holds every run-state backend to one lifecycle contract. The scheduler no longer waits for a run; a lost completion is detected by lease expiry and retried; so no single job can stall the scheduler.
status: review
---

## Symptom (Atlas, postgres build)

Scheduled tasks timed out waiting for their result ("queue reported no result
within 20m"), then failed on every retry with "scheduler: an identical task job
is already pending". The server stayed up; scheduled work stopped. A restart
cleared the immediate blockage.

## Reproduced cause

neoq's postgres backend runs each handler inside the transaction that holds the
job's `FOR UPDATE` row lock. rela does not pass
`postgres.WithTransactionTimeout`, so neoq's default
`idle_in_transaction_session_timeout = 30000ms` applies to every worker
connection. A handler that runs longer than 30 s leaves that transaction idle,
so PostgreSQL terminates the session. Then:

1. The handler finishes, but `updateJob` fails with `FATAL: terminating
connection due to idle-in-transaction timeout (SQLSTATE 25P03)`.
2. The row keeps `status = 'new'` and `retries = 0`.
3. neoq's pending-job poll (60 s) announces it again and it RUNS AGAIN, with no
limit. `retries` never increments, so `RetryNever` never trips.
4. The row never leaves the partial unique index on `(queue, fingerprint, status)`,
so every later enqueue with `IdempotencyKey: task.Name` returns
`ErrDuplicateJob` → `errTaskPending`.

Local repro against PostgreSQL 18 with a 45 s handler on `NewPostgresQueue`: the
job ran 3 times in 160 s and every re-enqueue was rejected as already pending.
Each rerun repeats the script's side effects (mail, HTTP, writes).

## Why the scheduler stalls

`enqueueTask` blocks the single scheduler goroutine on an in-process channel for
up to `taskResultTimeout` (20 m). Any path that loses the result blocks every
other task for 20 m per attempt: a job executed by another node, a job rerun
after a restart, a handler that outlives its 15 m `JobTimeout`, or a worker pool
saturated by the endless reruns above. After the timeout, the retry ladder
retries into the idempotency key held by the stuck row.

## Scope

- The 30 s idle-transaction kill affects EVERY durable job kind, not only the
scheduler (mail, AI, for_each children).
- The synchronous wait and the process-local completion channel are the
scheduler-side defect (overlaps TKT-7XLVP7 and TKT-DK0X6O).

## Verify on Atlas

- Server log: `terminating connection due to idle-in-transaction timeout` and
`error updating job status` from neoq.
- `SELECT id, status, retries, created_at, ran_at FROM neoq_jobs WHERE status <> 'processed'`
in the tenant schema: a scheduler row with `status = 'new'`, `retries = 0` and a
`created_at` far in the past.

## Fix plan (decided)

### Part 1: queue seam

- Pass `postgres.WithTransactionTimeout` above `handlerTimeout`, so neoq's
handler timeout always fires before PostgreSQL kills the transaction.
- Route neoq's logger through rela's logger (`component=neoq`). It currently
writes plain text to stdout with its own handler.
- Expose the attempt number to handlers (`jobs.Job.Attempt`, plus whether it is
the final one), and document that delivery is at-least-once.
- Regression test AM-durable-job-outlives-idle-tx.

### Part 2: durable scheduled-run lifecycle

Extends `internal/schedulerstate` (the unwired TKT-DK0X6O package) with run
records. Backends: `kvstate` over `state.KV` (fs, desktop, sqlite; single
process) and a new postgres backend in the tenant schema.

- The tick never waits. Per task it reads state and the active run. An active
run means skip; this is per-task backpressure for every schedule, and skipped
slots merge into one.
- Run creation is conditional on the task-state version the tick read and on
"no active run for the task" (a unique partial index on postgres). Two nodes
ticking at once create one run.
- The job's idempotency key is the run id, never the bare task name.
- Worker start: `queued -> running` compare-and-set with a lease. A run that is
already finished is a duplicate delivery; it is logged and skipped.
- Worker end: run status and task state (last success or the retry ladder) are
written in one transaction by the node that executed it.
- Lease: fixed, `handlerTimeout` plus margin, no heartbeat. `for_each` progress
(each child start or settle) extends it.
- Reaper, each tick: an expired `queued` or `running` run becomes `abandoned`
and advances the ladder. Replaces `taskResultTimeout`.
- `for_each`: the run finishes when every child has settled. Each child settles
once (idempotent per subject), on success or on its final attempt. Any failed
child fails the run (fixes BUG-1YMHIS).
- Time is injected into the store, as the schedulerstate contract already
requires. Lease margins cover clock skew between nodes.
- Logging: `run queued`, `run started` (queue wait, node), `run finished`
(duration, outcome), `run abandoned` (ERROR), duplicate delivery (WARN), all
with `task` and `run_id`.

Deleted: `claimInFlight`, `releaseInFlight`, `reportInFlight`, the run token,
`errTaskInFlight`, `errTaskPending`, `taskResultTimeout`, the in-memory `State`.

Out of scope: `RunTaskNow` (TKT-NLWV9P) builds on this by waiting for a run
record to finish; `for_each` on interval schedules stays rejected.

## Implemented

Part 1 landed as planned. The regression test is
`TestPostgresQueue_HandlerOutlivesDefaultIdleTxTimeout` in
`internal/jobs/pgqueue_test.go`.

Part 2 differs from the plan in these points:

- Leases: 30 m while queued, 20 m once running. The queued lease covers queue
wait on a busy pool; the running lease exceeds the 15 m handler timeout.
- Backends: postgres builds use `pgschedstate` (migration 0017:
`scheduler_tasks`, `scheduler_runs`, `scheduler_run_children`). Every other
build uses `kvstate` over `state.KV`, key `scheduler-run-state.json`. The run
state reaches exactly as far as the job queue does.
- A retried `for_each` run skips subjects that an earlier run of the same
occurrence already delivered (`SucceededSubjects`), so recipients are not mailed
twice.
- The legacy `scheduler-state.json` is imported once at start and deleted.
- Ended runs are pruned after 14 days, hourly.
- On the ephemeral queue (fs, desktop), a restart loses queued jobs. The task
then waits for the lease to expire before it retries. Accepted: the queue is
ephemeral by design, and a restart rarely falls inside a run.

Also fixes BUG-1YMHIS: a failed child now fails the `for_each` run.
