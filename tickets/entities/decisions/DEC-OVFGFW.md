---
id: DEC-OVFGFW
type: decision
title: 'No leader election for the scheduler: dedup plus persisted lastRun already give what a leader would'
context: Multi-node rela-server deployments raised the question of electing one primary scheduler node, possibly via a PostgreSQL lock or lease. Researched Oban, River, Solid Queue, Quartz, Celery Beat and neoq against primary sources, and measured PostgreSQL advisory-lock behaviour directly.
consequences: 'The scheduler stays leaderless: every node ticks, the queue''s fingerprint unique index arbitrates submission, and missed runs are detected from the persisted lastRun. No lease table, no advisory lock held across a term, no pinned connection per node. Revisit only if rela grows periodic work that cannot be fingerprinted.'
date: "2026-08-25"
status: accepted
---

## Decision

**Do not add leader election to the scheduler.** Every node continues to tick;
correctness comes from two mechanisms rela already has.

## Why

**1. Submission dedup is already atomic, and stronger than a leader.**

The scheduler submits with `IdempotencyKey: task.Name`, which becomes
`sha256(kind ‖ "\0" ‖ task.Name)` — no node id, no timestamp, no randomness, so
every node derives the same fingerprint from the same `schedules.yaml`. neoq
stores it under a partial unique index:

```sql
CREATE UNIQUE INDEX neoq_jobs_fingerprint_unique_idx
  ON neoq_jobs (queue, fingerprint, status) WHERE NOT (status = 'processed');
```

Verified empirically: two queue instances against one database, both enqueuing
on the same tick → one accepted, one `ErrDuplicateJob`, **exactly one
execution**, and the key freed after completion so the next occurrence is not
blocked.

That is a DB-arbitrated guarantee with no TTL, no clock and no failover window.
Leader election is a *probabilistic* mutex with an uncertainty band at every
transition — a weaker guarantee layered on a stronger one.

**2. Missed-run detection already exists, and is what the others lack.**

`Schedule.IsDue(lastRun, now)` compares against durable state
(`truncateToDay(now) != truncateToDay(lastRun)`, `now.Sub(lastRun) >=
interval`). A desktop app closed for three days still runs its daily task on
launch. This is the property that made neoq's `StartCron` unusable for rela, and
it is the thing the alternatives do *not* have:

- **River** has full OSS leader election and still documents that periodic jobs
*"will sometimes be skipped"*, because the leader's schedule state is in-memory:
a new leader restarts every schedule from "now". Their advice is to combine
periodic jobs with **unique** jobs; durable run times are a River Pro feature.
- **Solid Queue** keys recurring runs on `(task_key, run_at)`, but the row is
created at fire time by an in-process timer (`schedule_task(task, run_at:
task.next_time)`). It is concurrency control, not history — a cluster down at
09:00 never inserts the row and nothing later notices.

So the two systems most often cited as prior art each have one half of what rela
already has both of.

**3. The PostgreSQL primitives do not offer what the question assumed.**

Advisory locks have **no TTL**. Postgres docs: *"an advisory lock is held until
explicitly released or the session ends."* Measured directly: a lock was still
held **60+ seconds** after `kill -9` of the client, because the backend sat idle
on a dead TCP peer, and `tcp_keepalives_idle = 0` by default means the server
may never probe. A lease TTL is a number you choose; advisory-lock release
latency is a number the kernel chooses.

Session-scoped locks also gate only the holding connection, so a leader would
have to pin a pgxpool connection for its whole term — the same
one-connection-per-node cost CLAUDE.md already rejects for LISTEN/NOTIFY (*"the
term that does not shrink under pooling"*).

A lease table avoids both problems (prototyped: `INSERT ... ON CONFLICT DO
UPDATE ... WHERE expires_at < now()` with a term as fencing token, verified to
transfer on expiry and fence a zombie leader). It is what **Oban** uses — they
moved off advisory locks in v2.11 for exactly these reasons, and **River** uses
the same shape. But it is a new table, a heartbeat goroutine, a TTL, jitter and
a trust window — to serialize work the unique index already serializes.

## What was actually wrong

The real defect is not the absence of a coordinator. It is that scheduler
run-state is one JSON blob written through a KV whose `Put` is an unconditional
upsert: `loadState` reads once at startup, `saveState` rewrites every task, and
two nodes silently clobber each other. A leader would *hide* that bug by making
one writer usual, not fix it. See TKT-DK0X6O.

## Consequences accepted

- **Redundant tick work.** N nodes each evaluate every schedule; N−1 lose the
insert race. For cron arithmetic plus one insert this is negligible, and it
scales with nodes × schedules, not with work.
- **No single row answering "who runs cron?"** Mitigated cheaply by recording
the winning node's id on the dedup row, which answers the more useful question —
*which node actually ran this tick* — as a column rather than a subsystem.
- **The unique index is load-bearing.** Solid Queue #292 is the cautionary tale:
a missing index silently stops deduplicating. It should be asserted, not
assumed.

## Revisit if

rela grows periodic work that **cannot be fingerprinted** — bulk pruning, a
reindex, anything without a stable identity. Note the existing precedent for
that case is the postgres version sweep: `pg_try_advisory_lock` on one pinned
connection for the duration of **a tick**, never a term. A short bounded
operation never hits the no-TTL problem. That is the shape to copy, not
leadership.
