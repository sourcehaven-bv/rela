---
paths:
  - "internal/jobs/**"
  - "internal/scheduler/**"
  - "internal/schedulerstate/**"
---

# Background jobs

- **Background jobs: the queue knows nothing about schedules, and never runs
  before a transaction closes.** External side effects (mail, HTTP, AI) belong
  on `jobs.Queue` rather than inline on a write path. Two rules keep the seam
  usable:

  _Retry is a flat enum_ (`RetryNever` / `RetryBounded` / `RetryPersistent`),
  plus an optional deadline and idempotency key — nothing else. The enum names
  INTENT; mechanism (attempt counts, backoff, the `RetryPersistent` outer bound)
  lives in `internal/jobs/retry.go` and is meant to be retuned there for
  everyone. Do NOT widen it into a policy struct or add per-call knobs: a call
  site needing different mechanics is evidence for a new intent value.

  _A recurring task uses `IdempotencyKey`, never a cadence-derived `Deadline`._
  A key says "one of these pending at a time is enough", so a run that is still
  queued suppresses the next rather than stacking a second copy — a daily report
  delayed six hours must not then send twice. A deadline expresses something
  different: "this is worthless after T", which makes the job VANISH when it
  cannot start in time. Under load that drops scheduled work precisely when the
  operator most wants it done, and (before the guard existed) hung the scheduler
  on a completion that never arrived. Deadlines are for work whose value
  genuinely expires; schedules are not that.

  The scheduler itself keys each job by its RUN id, not by task name
  (BUG-TKL08E). "One run per task at a time" is enforced by the run-state
  store (`internal/schedulerstate`), which every node can query. A task-name
  key made the queue a second, invisible source of truth: a job row the queue
  could not complete held the key forever and blocked every later run. Do not
  move non-overlap back into the queue key.

  _A job enqueued inside `store.Store.Tx` must not become runnable until that
  transaction commits._ Otherwise a worker reads it on another connection that
  cannot see the uncommitted writes and acts on the pre-write world — a race
  that passes tests and fails under load. `jobs.WithDeferral` collects enqueues;
  the transaction seam calls `Flush` on commit or `Discard` on rollback,
  mirroring pgstore's `txPending`. Pinned by `jobstest`.

  The fs/desktop tier is EPHEMERAL on purpose — jobs vanish on exit, because an
  unsent mail from an ended session is not worth resurrecting. Don't "fix" it to
  persist; that is what the postgres tier is for.

  _The durable queue's tables live in the TENANT's schema, like every other
  postgres-backed table._ A schema-pinned `search_path` is how rela scopes a
  tenant, and the queue is not exempt: rela submits every kind to one queue name
  and neoq's insert trigger does `pg_notify(NEW.queue, ...)`, so tables shared
  across tenants would mean tenants consuming each other's jobs. neoq v0.72.1
  could not do this — one migration named `public.neoq_jobs_id_seq` while its
  tables follow `search_path` — which is why `go.mod` carries a `replace` onto a
  fork (BUG-YJEIFH, upstream acaloiaro/neoq#149). Drop the `replace` when that
  lands, not before: `TestPostgresQueue_SchemaPinnedDSN` is what fails if it
  goes early. **Test any new postgres-touching dependency through a
  schema-pinned DSN**, not just the bare `RELA_TEST_DATABASE_URL` — the bare DSN
  resolves to `public`, which is precisely the one case that worked.
