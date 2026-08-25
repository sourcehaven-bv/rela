---
id: TKT-DK0X6O
type: ticket
title: 'Scheduler run-state gets its own storage service: per-task rows, atomic outcome writes, out of the general KV blob'
kind: enhancement
priority: medium
effort: m
tags: needs-design
status: ready
---

## Problem

Scheduler run-state lives in a single JSON document — `scheduler-state.json` —
written through `state.KV`. `State` holds three maps keyed by task name
(`Tasks`, `Failures`, `NextRetry`), and `saveState` marshals **all of them, for
every task**, on every write (`internal/scheduler/scheduler.go`).

**The defect is the lost update.** `loadState` reads once at startup; thereafter
`s.state` is in-memory, and `StateKV.Put` is an unconditional `INSERT ... ON
CONFLICT DO UPDATE SET value = EXCLUDED.value`. Two nodes each holding a stale
snapshot overwrite each other's tasks **wholesale** — not just the contended
task, every task. `recordSuccess` states the assumption plainly: `// No mutex
needed, single goroutine.` That was true. The blob *is* the concurrency
assumption, written down as a data shape.

Note on size, measured so it is not overstated: at 50 tasks all mid-ladder the
document is ~5.4 KB, and ~24 KB at 500 healthy tasks, against `state_kv`'s 32
MiB cap. Rewriting it per tick is wasteful but not a real cost. **Do not justify
this ticket on blob growth** — the justification is that a write about one task
must not carry, and cannot clobber, every other task's state.

## Approach: a per-task storage service

A **scheduler-owned service** at the grain of the actual mutations — one task,
one record. Every operation is already per-task:

| Operation | Touches |
|---|---|
| `recordSuccess` | one task: set last-run, clear ladder |
| `recordFailure` | one task: bump failures, set next-retry |
| `IsDue` | one task, read-only |
| `pruneOrphanedState` | housekeeping only |

Interface, after design review (see the review-responses below — the first draft
had three critical defects):

```go
// RunState is one task's scheduling record.
type RunState struct {
    LastRun   time.Time // last SUCCESSFUL start; zero = never run
    Failures  int       // consecutive failures; 0 = healthy
    NextRetry time.Time // zero = no retry pending
}

type Store interface {
    // Load reads state for the named tasks. Scoped, not All(): unbounded
    // reads of a shared table would return other deployments' rows, and
    // scoping means an orphaned record is simply never read.
    Load(ctx context.Context, tasks []string) (map[string]RunState, error)

    // RecordSuccess stamps the run's START time and CLEARS the ladder, in
    // one atomic write. The clear is load-bearing: it is the only reset
    // (RR-Y4417I).
    RecordSuccess(ctx context.Context, task string, start time.Time) error

    // RecordFailure increments the failure count atomically and returns the
    // new value; the caller derives the delay. Guarded so it cannot clobber
    // a success that started later (RR-R43942, RR-UUWC92).
    RecordFailure(ctx context.Context, task string, start time.Time) (failures int, err error)

    // SetNextRetry records when the task may next be attempted, guarded by
    // the same start-time predicate.
    SetNextRetry(ctx context.Context, task string, start, retryAt time.Time) error

    // Prune drops records untouched since `before`. Housekeeping only:
    // orphaned rows are never read, so a backend that never prunes stays
    // CORRECT while growing (RR-JKT6PZ).
    Prune(ctx context.Context, before time.Time) ([]string, error)

    Close() error
}
```

Deliberate properties, each traceable to a finding:

- **The ladder stays in `internal/scheduler`.** `retryDelay` is a pure function
of the failure count and is scheduling *policy*; the store owns the atomic
increment, never the curve. Two calls on the failure path (increment, then set
retry) is fine — failure is the rare path and the ladder's floor is 5m.
- **Both write paths carry the run's START time** and are guarded on it, so a
stale node can neither regress a newer last-run nor clobber a newer success.
- **No wall clock inside the store.** `now` is always supplied, following
`userstate`'s rule and for the same reason: deterministic conformance tests
across backends.
- **The clock-jump clamp is read-time, not stored** — a pure
`clampRetry(rs, now)` applied after `Load`. It is idempotent and operates on
untrusted input (hand edits, VM resume, NTP step), so it must not depend on
having been written back (RR-DPNJO0).

Backends:

- `internal/schedulerstate/kvstate` — one document via `state.KV`, as today.
Single-writer by nature; honest about it, like `kvuserstate`.
- `internal/store/pgstore/schedulerstate.go` + a numbered migration — one row
per task, `PRIMARY KEY (task)`, guarded conditional updates.
- `internal/schedulerstate/schedulerstatetest` — `RunAll(t, factory)` against
both, the postgres arm DB-gated and wired into `just test-postgres` **and the CI
postgres job** (the gap TKT-YOED3R closed for `internal/jobs`; do not repeat
it).

## Migration is not rolling-deploy-safe

Importing and retaining the legacy key gives only *point-in-time* rollback: an
old binary (document) and a new one (rows) write different locations and diverge
permanently, running every task on both at both cadences (RR-9XGRXW).

So: **import once, then delete the legacy key.** An old binary then sees no
state and treats everything as first-run — loud and obvious rather than silent
divergence. Document "stop schedulers, deploy, start" in
`docs/postgres-backend.md`. The scheduler is a single sequential component, so a
brief stop is far cheaper than duplicate mail sends.

## Prior art

`internal/userstate` (TKT-CXD0A4) is the accepted precedent for per-record state
deliberately outside `store.Store`, and its KV backend documents this exact bug:
*"two processes can read the same document, each apply their own change, and the
second write clobbers the first ENTIRELY — not just the contended key."* Its
postgres migration states the tier split in the same terms. The package must
earn the same exemption in prose: why this is not in the graph, backends +
conformance suite, time injected never read, `ErrClosed`, and `Nil:` contract
tags.

## What this ticket does NOT do

- **The executing node recording the outcome** — that is TKT-7XLVP7, which
depends on this. This ticket makes the write *safe*; the follow-up moves *who*
calls it. **Until then this fixes clobbering, not exactly-once**: both nodes
still execute every task and merely agree on the bookkeeping afterward.
- **Leader election** — rejected, DEC-OVFGFW.
- **`(task_key, run_at)` occurrence keying** — rejected; `lastRun` already
gives catch-up, which that pattern does not.
- **CAS on `state.KV`** — rejected: widens a seam the render cache, settings,
logo and CalDAV aliases share, and puts a retry loop into a component whose
defining property is that execution is sequential.

## Acceptance

1. **Per-task writes.** `RecordSuccess("a", …)` leaves task `b`'s record
unchanged.
2. **Success clears the ladder atomically.** After `RecordSuccess`, `Failures`
is 0 and `NextRetry` is zero — pinned by a conformance case, because getting
this wrong re-opens BUG-ZKK2UL.
3. **No regression.** Out-of-order `RecordSuccess` leaves the newest start.
4. **A failure cannot clobber a newer success.** A `RecordFailure` whose start
predates a recorded success is a no-op.
5. **Failure count increments atomically** and never moves backwards.
6. **Concurrent writers lose nothing.** N goroutines on separate connections,
distinct tasks; all N records present. This is the test that fails today.
7. **Migration preserves in-flight ladders**, and the legacy key is gone
afterwards.
8. fs/desktop behaviour is unchanged; existing scheduler tests pass untouched.
9. The `state_kv` migration header no longer claims scheduler bookkeeping.
