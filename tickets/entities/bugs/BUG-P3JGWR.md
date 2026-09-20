---
id: BUG-P3JGWR
type: bug
title: Retry ladder is stamped from the run's start, so a slow failure retries in the past and the scheduler wedges
description: 'recordFailure computed retryAt as start+delay. A run that fails only after delay has already elapsed — the 20m taskResultTimeout against the 5m first rung always does — gets a retry stamped in the past, so it is due on every tick. Each retry then finds the previous run still pending and returns without recording anything, pinning failures at 1 so the ladder never climbs and the ERROR escalation is never reached. Observed in production: every scheduled task stopped running for 31h while the service stayed healthy and no alert fired.'
priority: high
effort: s
why1: A failing task re-ran once per 60s tick indefinitely, and every other scheduled task stopped running with it, while the process stayed up and reported healthy.
why2: recordFailure stamped retryAt as start.Add(delay). With a 20m taskResultTimeout and a 5m first rung, the retry was written 15 minutes BEFORE the failure was observed, so runDueTasks found it due on the very next tick and every tick after.
why3: recordSuccess deliberately stamps the run's START time, so a long run cannot drift the schedule forward, and that reasoning is documented on the function. It was carried across to recordFailure, where it inverts the intent — a backoff must be measured from when the failure was observed, not from when the attempt began.
why4: Nothing downstream could catch it. The clock-jump guard in runDueTasks clamps only retries too far in the FUTURE; a retry in the past is exactly the case it does not cover. The backoff ladder would normally escalate to ERROR at four consecutive failures, but every retry took the skip path in doExecuteTask, which records neither success nor failure, so failures stayed at 1 forever and the escalation was unreachable.
why5: 'The skip path is right for the case it was written for and wrong for one it cannot distinguish. A run already pending means either a healthy task running long or a wedged task whose previous run never cleared; the scheduler sees the same sentinel for both. Choosing "record nothing" made the healthy case correct and the wedged case permanently silent — and because the wedge blocks the shared queue fingerprint, one stuck task takes every other task with it.'
prevention: 'Three regression tests pin the corrected behaviour, two of which fail against the old code with the exact production symptom: a retry stamped before the failure that produced it, and a failure count that never advances. The existing test asserted retryAt == start+delay and so pinned the defect; it passed only because its injected clock made the run instantaneous, which hid the start-vs-now distinction entirely. More generally: a test that fixes elapsed time at ~0 cannot tell those two clocks apart, so any duration-derived value needs a case where elapsed is larger than the value being derived.'
status: backlog
---

## Description

`recordFailure` in `internal/scheduler/scheduler.go` computed the next retry
from the run's start time:

```go
failures := s.state.Failures[task.Name] + 1
delay := retryDelay(failures)
retryAt := start.Add(delay)
```

`start` is captured before `enqueueTask` blocks. When the enqueue gives up
after `taskResultTimeout` (20m) and the first rung is `baseRetryDelay` (5m),
the retry is stamped 15 minutes before the failure was observed. It is
therefore already due, and `runDueTasks` fires it on the next tick.

The clock-jump guard immediately above the due check only clamps a retry that
is implausibly far in the *future*:

```go
if retryAt.Sub(now) > maxRetryDelay { ... }
```

A retry in the past passes straight through.

## Why it never escalated

Each retry re-enqueues, finds the previous run still pending, and takes the
skip path in `doExecuteTask`, which returns without touching state. That is
deliberate and correct for a healthy-but-slow task. Here it meant
`Failures` stayed at 1 and `NextRetry` stayed pinned at the same past stamp,
so the ladder could not climb through 10m/20m/40m and
`persistentFailureThreshold` could never promote the log line to ERROR.

The result is the worst shape for an operator: a task that is definitively
broken, logging at INFO/WARN once a minute, with a process that reports
healthy.

## Observed state

A live scheduler in this condition:

```json
{
  "tasks": {
    "recurrence": "2026-09-18T00:00:18Z",
    "taak-validatie-review": "2026-09-18T00:00:18Z",
    "mt-agenda": "2026-09-16T00:00:22Z"
  },
  "failures":   {"recurrence": 1},
  "next_retry": {"recurrence": "2026-09-19T00:05:35Z"}
}
```

The failure that wrote `next_retry` happened at `00:20:35` — fifteen minutes
after the timestamp it wrote. Note that **all three** tasks stopped, not just
the one that failed: the stuck run holds the queue fingerprint, so every
later enqueue for any task collapses into it.

## Fix

Two changes in `internal/scheduler/scheduler.go`.

1. `recordFailure` measures the ladder from the observed failure time:
   `start.Add(elapsed).Add(delay)`. Using `start.Add(elapsed)` rather than
   `s.now()` keeps the function deterministic under the injected test clock
   and reuses the `elapsed` the caller already passes.

2. The skip path advances the ladder when a retry is already armed. A skip
   with no retry armed still records nothing, preserving the
   healthy-slow-task behaviour that motivated the branch.

With both applied the sequence becomes fail → retry 5m later → skip advances
to 2 → 3 → 4 (ERROR) → capped at 2h, instead of one skip per tick forever.

## Out of scope

Neither change clears an orphaned queue row, so a task already wedged stays
wedged until the row is cleared. The durable fix — letting the executing node
own `recordSuccess`/`recordFailure` so there is no submitter wait, no
`taskResultTimeout`, and no pending sentinel — is already described in
`internal/scheduler/jobs.go` as TKT-7XLVP7 / TKT-DK0X6O.

This bug also has no monitoring counterpart: a liveness probe on the process
cannot see a scheduler that is alive and doing nothing. A check on the age of
the newest entry in the `tasks` map belongs with the deployment config, not
here.
