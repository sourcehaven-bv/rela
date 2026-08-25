---
id: TKT-DK0X6O
type: ticket
title: 'Scheduler run-state gets its own storage service: per-task rows, atomic outcome writes, out of the general KV blob'
kind: enhancement
priority: medium
effort: m
tags: needs-design
status: planning
---

## Problem

Scheduler run-state lives in a single JSON document — `scheduler-state.json` —
written through `state.KV`. `State` holds three maps keyed by task name
(`Tasks`, `Failures`, `NextRetry`), and `saveState` marshals **all of them, for
every task**, on every write (`internal/scheduler/scheduler.go`).

So a write about `daily-report` carries a full copy of `weekly-cleanup`'s state
with it. Three consequences, in increasing order of severity:

1. **The blob grows with the task count** and is rewritten in full on every
success and every failure. `state_kv` values are `BYTEA` read and written whole,
capped at 32 MiB — a limit sized for uploaded logos and cached renders, not for
a hot path that writes once per task per tick.
2. **The read is a startup snapshot.** `loadState` runs once in `Run`;
thereafter `s.state` is in-memory. A second process never sees the first's
writes.
3. **Concurrent writes silently lose data.** `StateKV.Put` is an unconditional
`INSERT ... ON CONFLICT DO UPDATE SET value = EXCLUDED.value` — last writer
wins, no version, no CAS. Two nodes each holding a stale snapshot overwrite each
other's tasks wholesale.

`recordSuccess` says it plainly: `// No mutex needed, single goroutine.` That
was true. The blob *is* the concurrency assumption, written down as a data
shape.

## Why not just add CAS to state.KV

Because `state.KV` is shared with the document render cache, user settings, the
operator logo/theme and the CalDAV alias table. Adding compare-and-swap to that
interface widens a seam four unrelated consumers depend on, to fix one — and
every backend (`FSKV`, `pgstore.StateKV`, plus `state.ValidatedKV` and the
`statetest` conformance suite) would have to implement and be tested against a
primitive only the scheduler wants.

The `state_kv` migration header currently lists "scheduler bookkeeping" among
its tenants. **This ticket deliberately reverses that**, and the header should
be updated to say so — the general KV is right for whole-value blobs (a logo, a
rendered document) and wrong for a record with independently mutating fields.

## Approach

A **scheduler-owned storage service** at the grain of the actual mutations — one
task, one record. Every operation is already per-task:

| Operation | Touches |
|---|---|
| `recordSuccess` | one task: set last-run, clear ladder |
| `recordFailure` | one task: bump failures, set next-retry |
| `IsDue` | one task, read-only |
| `pruneOrphanedState` | set difference against configured tasks |

A narrow consumer-side interface (declared at the call site, per CLAUDE.md) of
roughly `Get(task) → RunState`, `RecordSuccess`, `RecordFailure`, `Prune`. Not a
repository abstraction over the store — a domain service the scheduler owns, in
the shape of `caldavalias.Service`.

Per-tier backing:

- **postgres**: its own table, so a write is one row. `RecordSuccess` becomes a
conditional `UPDATE ... WHERE last_run < $new`, which makes a stale node unable
to regress a newer result.
- **fs/desktop**: per-task keys under `state.KV`, or a small file per task.
Single-writer by nature, so the guarantee is trivially satisfied.

**Migrate the existing blob on first read** rather than resetting ladders: a
mixed-version rollout must not lose in-flight retry state. The existing
`parseState` backward-compatibility note in `state.go` describes the same hazard
for an older binary.

## The other half: the executing node records the outcome

Granularity alone does not fix the cross-node bug. Today the node that
*enqueued* a task waits for the result and writes the outcome — but any node's
worker may claim the job (`FOR UPDATE SKIP LOCKED`), so node A can record a
result for work node B ran, and after TKT-YOED3R's `taskResultTimeout` it
records a **failure for a task that succeeded**.

So `recordSuccess` / `recordFailure` move into `runTaskJob`, on the node that
actually ran the script. That is what makes the per-task write meaningful, and
it removes the completion channel, `claimInFlight`/`releaseInFlight`/
`reportInFlight`, both skip sentinels, the run-token machinery (RR-6INQHE) and
`taskResultTimeout` — all of which exist only to support the synchronous wait.

## What this ticket does NOT do

**No leader election.** Researched against primary sources and deliberately
rejected. Two facts settle it: rela already dedups submissions atomically (a
fingerprint under a partial unique index — verified with two queue instances
against one database, same tick → 1 execution), and rela already detects missed
runs from the persisted `lastRun` via `IsDue`, which is what River charges for
in River Pro and Solid Queue does not do at all. River has full leader election
and *still* documents that periodic jobs "will sometimes be skipped", because
its schedule state is in-memory per node. The open problem here is unsafe
read-modify-write on shared state, not the absence of a coordinator.

**No `(task_key, run_at)` occurrence keying.** Considered, from Solid Queue, and
rejected: its `run_at` row is created at fire time by an in-process timer
(`schedule_task(task, run_at: task.next_time)`), so it is concurrency control,
not history — a cluster that is down at 09:00 never inserts the row and nothing
later notices. rela's persisted `lastRun` is strictly stronger. Adopting the row
would trade catch-up for fire-time semantics.

## Design questions to settle first

- **Where does the fs/desktop implementation live** — per-task `state.KV` keys,
or a directory of small files? Keys are simpler and reuse `ValidatedKV`'s rules;
files match how the rest of `.rela/` works.
- **Does the postgres table need its own migration, or a typed view over
`state_kv`?** A table is cleaner and allows an index and a conditional update; a
view avoids a migration. Prefer the table — the conditional `UPDATE ... WHERE
last_run < $new` is the point.
- **Prune semantics under concurrency.** `pruneOrphanedState` drops state for
tasks no longer in `schedules.yaml`. Two nodes with *different* `schedules.yaml`
(mid-rollout) would each prune the other's tasks. Today the blob makes this
total; per-task rows make it survivable, but the rule needs stating — probably
"only prune what this node can see, and only after a grace period".
- **The clock-jump guard (BUG-ZKK2UL)** was written against a single writer.
Re-check `retryAt.Sub(now) > maxRetryDelay` under concurrent updates.
- **Does the fs tier pay any cost?** It is single-writer and gains nothing from
atomicity. Keep its implementation trivial; the complexity is bought for the
networked tier only.

## Acceptance

- Scheduler run-state is stored per task, not as one document.
- A success or failure writes only the affected task's record.
- On postgres, two processes running the scheduler against one database do not
lose each other's updates; a stale node cannot regress a newer last-run.
- The node that EXECUTES a task records its outcome; a task run on node B is
never recorded as failed by node A.
- Existing `scheduler-state.json` content is migrated, not discarded — in-flight
retry ladders survive the upgrade.
- fs/desktop behavior is unchanged.
- The `state_kv` migration header no longer claims scheduler bookkeeping.
- The SINGLE-NODE ONLY note in `internal/scheduler/jobs.go` is removed or
narrowed to whatever genuinely remains.
