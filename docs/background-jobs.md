# Background jobs

Rela talks to external systems — outbound mail, remote HTTP APIs, LLM
providers. Those calls are slow and fail intermittently. Running them inline on
a write path means a third-party timeout becomes your timeout, and every caller
invents its own retry logic.

Background jobs move that work off the calling goroutine and give it one retry
policy.

## Durability depends on how you deploy

Rela does not force one durability guarantee on every deployment, because the
deployments genuinely differ:

| Deployment | Backend | What happens to queued work |
| --- | --- | --- |
| Local / desktop (filesystem store) | in-process | Lost when the process exits |
| Networked server (PostgreSQL store) | PostgreSQL | Survives restarts; shared across server processes |

**Losing queued work on exit is intended behaviour for the local tier, not a
limitation.** A desktop session that ends with an unsent notification should
not resurrect it three days later on next launch. If you need work to survive a
restart, you want the PostgreSQL backend — which is the same switch that gives
you a networked, multi-process deployment.

No configuration selects this. The backend follows the store: build with the
`postgres` tag and set `RELA_DATABASE_URL`, and jobs become durable along with
everything else.

## Retry policy

A job declares how much effort it is worth, and nothing more:

| Policy | Meaning |
| --- | --- |
| `RetryNever` | Run once. A failure is final. |
| `RetryBounded` | Try a few times, then give up. |
| `RetryPersistent` | Keep trying; this should get through eventually. |

That is the whole vocabulary. There is deliberately no way to specify an
attempt count or a backoff curve at the call site: those are chosen centrally so
they can be tuned in one place rather than across every producer.

`RetryPersistent` does not mean literally forever. Work that has been failing
for roughly two days is dropped with an error log — at that point it needs a
person, not another attempt.

A job may also carry a **deadline**: a time after which it is no longer worth
running. A job already past its deadline is discarded rather than attempted.

## Scheduled tasks and retries

`schedules.yaml` is unchanged, and scheduled tasks keep their existing
behaviour: a task that fails is retried on a backoff ladder that suppresses its
normal cadence until it succeeds.

What the job system adds is a sensible interaction between the two: **there is
no point retrying a task whose next run is imminent.** A task on a 60-second
cadence should not sit in a five-minute backoff — the next run will come first
and do the same work.

The scheduler expresses this by handing the queue its own next scheduled run as
the job's deadline. A short-cadence task therefore stops retrying just before
its next run re-submits it, while a daily task gets a real backoff window inside
its 24 hours. The queue itself knows nothing about schedules; it only
understands deadlines.

## Ordering with respect to writes

A job scheduled during a write is not started until that write's transaction
commits.

This matters more than it sounds. Without it, a worker could pick the job up on
a different database connection — one that cannot see the not-yet-committed
write — and act on a world where the entity does not exist yet. The result is a
notification about a missing record, or one carrying stale values. Because it
depends on timing, it tends to work in testing and fail under load.

If the transaction rolls back, its jobs are discarded.

## Duplicate submissions are distinct jobs

Submitting the same payload twice produces two jobs and two executions. The
queue does not deduplicate by payload contents — two triggers that both decide
to notify the same person will send two notifications.

This is worth stating because the opposite is a common queue default, and
relying on it silently would mean losing work.

## What this does not do

Job scheduling is not atomic with the database write. If the process dies in the
narrow window between a commit and the job being queued, that job is lost. This
matches how Redis- and AMQP-backed queues behave alongside a database, and it is
the right trade for rela's workloads: job payloads here are notifications and
API calls, where a rare miss is an annoyance rather than a correctness problem.

Handlers should be written accordingly — re-check current state rather than
trusting a payload to still be accurate, and make repeated delivery harmless
where it matters.

A single handler execution is also capped at 15 minutes. That is a backstop
against a wedged handler holding a worker forever, not a latency target: real
bounding comes from the job's deadline and its retry budget.
