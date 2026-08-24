---
id: TKT-7XLVP7
type: ticket
title: 'Multi-node scheduler: move task bookkeeping to the executing node and scheduler state into state.KV'
kind: enhancement
priority: medium
effort: m
tags: needs-design
status: backlog
---

## Problem

`docs/postgres-backend.md` describes running several `rela-server` processes
against one database. The job queue supports that; the **scheduler does not**.

The confusion worth clearing up first: **jobs do not double-fire.** The
scheduler submits with `IdempotencyKey: task.Name`, which becomes a SHA-256
fingerprint stored under a partial unique index in neoq's schema:

```sql
CREATE UNIQUE INDEX neoq_jobs_fingerprint_unique_idx
  ON neoq_jobs (queue, fingerprint, status)
  WHERE NOT (status = 'processed');
```

Two nodes ticking simultaneously both INSERT; PostgreSQL admits exactly one and
the loser gets a unique violation, normalized by the seam to
`jobs.ErrDuplicateJob` → `errTaskPending` → "skip, neither success nor failure".
That is atomic in the database, not a check-then-act race. Dedup is sound.

What breaks is the **bookkeeping around** the job.

## Two concrete failures

**1. Cross-node results are lost, and now recorded as failures.**

Workers claim with `FOR UPDATE SKIP LOCKED`, so any node may execute any job:
node B can run a task node A enqueued. `enqueueTask` (node A) blocks on a
completion channel in A's *in-process* in-flight map. B's `reportInFlight` looks
in B's map, finds nothing, and drops the result — correctly, per its own
contract.

Node A then waits out `taskResultTimeout` (20m) and calls `recordFailure` for a
task that **succeeded**. Before TKT-YOED3R's timeout fix this hung A's scheduler
goroutine outright; the timeout converted an infinite hang into a
wrong-but-visible failure. On a single node that path is exceptional. On two
nodes it is routine.

**2. Split-brain last-run state.**

`.rela/scheduler-state.json` is node-local on the postgres tier. Each node keeps
its own last-run stamps and its own retry ladder, so "is this task due?" is
evaluated against a file the other node cannot see. The two views diverge
immediately and neither is authoritative.

## Approach

Stop having the submitter wait. Both failures come from the same root: the node
that *decides* a task is due also owns recording what happened, even though a
different node may have done the work.

- Move `recordSuccess` / `recordFailure` into `runTaskJob`, so the **executing**
node writes the outcome.
- Move scheduler state out of `.rela/` into `state.KV`, which is already
database-backed on the postgres build (TKT-VC27L3) precisely for node-local
state like the render cache and the operator logo. Rows live in the store's
schema, so schema-per-tenant scopes it for free.

That deletes the completion channel, `claimInFlight`/`releaseInFlight`/
`reportInFlight`, both skip sentinels (`errTaskInFlight`, `errTaskPending`), the
run-token machinery from RR-6INQHE, and `taskResultTimeout` — the whole
apparatus exists only to support the synchronous wait.

## Design questions to settle first

- **Concurrent state writes.** Two nodes updating the same task's state needs a
read-modify-write story. `state.KV` has no CAS today; the tick is already coarse
(60s) so an advisory lock around the state update may be enough. Check whether
`PatchEntity`-style merge semantics apply.
- **Who ticks?** With state shared, every node evaluating every task is
wasteful but not incorrect (the idempotency key absorbs the duplicates). A
leader election is the obvious alternative — probably not worth it, but it is
the decision to record.
- **The retry ladder becomes shared.** The clock-jump guard (BUG-ZKK2UL) was
written against a single writer; re-check it under concurrent updates.
- **The fs/desktop tier must not regress.** It has one node by definition, and
`state.KV` there is still filesystem-backed — behavior must be unchanged.

## Out of scope

- Leader election, unless the design questions force it.
- Any change to the `jobs.Queue` seam. The queue already does the right thing;
this is entirely a scheduler-tier concern.

## Acceptance

- Two `rela-server-postgres` processes against one database run a scheduled
task exactly once per cadence, and the node that executed it records the
outcome.
- A task executed on node B is not recorded as a failure by node A.
- Last-run and retry state agree across nodes.
- The single-node fs/desktop path is behaviorally unchanged.
- The SINGLE-NODE ONLY note in `internal/scheduler/jobs.go` is removed.
