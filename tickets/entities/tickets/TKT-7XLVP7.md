---
id: TKT-7XLVP7
type: ticket
title: 'Multi-node scheduler: the executing node records the outcome (depends on TKT-DK0X6O)'
kind: enhancement
priority: medium
effort: m
tags: needs-design
status: backlog
---

## Problem

`docs/postgres-backend.md` describes running several `rela-server` processes
against one database. The job queue supports that; the **scheduler does not**.

Two things are worth clearing up before the actual bug, because both were
initially misdiagnosed here.

**Jobs do not double-fire.** The scheduler submits with `IdempotencyKey:
task.Name`, which becomes `sha256(kind ‖ "\0" ‖ task.Name)` — no node id, no
timestamp, no randomness, so every node derives the same fingerprint from the
same `schedules.yaml`. neoq stores it under a partial unique index, so two nodes
ticking simultaneously both INSERT and PostgreSQL admits exactly one; the loser
gets `ErrDuplicateJob` → `errTaskPending` → "skip, neither success nor failure".
Verified with two queue instances against one database: **one execution**. Dedup
is sound, and no leader election is needed — see DEC-OVFGFW.

**Scheduler state is NOT node-local.** An earlier revision of this ticket said
it was. It is wrong: the scheduler reads and writes through `state.KV`
(`scheduler.go:88`), which on the postgres build is the `state_kv` **table**.
The state is already shared across nodes.

## The actual bugs

**1. The shared state is written unsafely.** `loadState` runs once at startup;
`saveState` then marshals the whole `State` struct — every task's last-run,
failure count and next-retry — and `StateKV.Put` is an unconditional upsert. Two
nodes each holding a stale snapshot silently clobber each other. **This is
TKT-DK0X6O**, which this ticket depends on.

**2. The wrong node records the outcome.** Workers claim with `FOR UPDATE SKIP
LOCKED`, so any node may execute any job: node B can run a task node A enqueued.
`enqueueTask` (node A) blocks on a completion channel in A's *in-process*
in-flight map; B's `reportInFlight` looks in B's map, finds nothing, and
correctly drops the result. Node A then waits out `taskResultTimeout` (20m) and
calls `recordFailure` for a task that **succeeded**.

Before TKT-YOED3R's timeout fix this hung A's scheduler goroutine outright; the
timeout converted an infinite hang into a wrong-but-visible failure. On a single
node that path is exceptional. On two nodes it is routine.

## Approach

Stop having the submitter wait. `recordSuccess` / `recordFailure` move into
`runTaskJob`, so the **executing** node writes the outcome — which is only safe
once the write is per-task and atomic, hence the dependency on TKT-DK0X6O.

That deletes the completion channel, `claimInFlight`/`releaseInFlight`/
`reportInFlight`, both skip sentinels (`errTaskInFlight`, `errTaskPending`), the
run-token machinery from RR-6INQHE, and `taskResultTimeout` — the whole
apparatus exists only to support the synchronous wait.

## Design questions to settle first

- **Who ticks?** With state shared and outcomes written by the executor, every
node evaluating every task is wasteful but not incorrect — the idempotency key
absorbs the duplicates. DEC-OVFGFW accepts that cost explicitly. Consider
recording the winning node's id on the job row so an operator can see *which
node actually ran this tick*.
- **The retry ladder becomes shared.** The clock-jump guard (BUG-ZKK2UL) was
written against a single writer; re-check it under concurrent updates.
- **`RunOnStart`-style behaviour on a cold cluster.** If every node restarts at
once, they all evaluate `IsDue` against the same shared `lastRun` — fine — but
confirm no thundering herd on a large schedule set.
- **The fs/desktop tier must not regress.** One node by definition; behaviour
must be unchanged.

## Out of scope

- **Leader election** — researched and rejected, see DEC-OVFGFW.
- **`(task_key, run_at)` occurrence keying** — considered from Solid Queue and
rejected; rela's persisted `lastRun` already gives catch-up, which that pattern
does not.
- Any change to the `jobs.Queue` seam. The queue already does the right thing.

## Acceptance

- Two `rela-server-postgres` processes against one database run a scheduled task
exactly once per cadence, and the node that executed it records the outcome.
- A task executed on node B is never recorded as a failure by node A.
- Last-run and retry state agree across nodes.
- The single-node fs/desktop path is behaviourally unchanged.
- The SINGLE-NODE ONLY note in `internal/scheduler/jobs.go` is removed.
