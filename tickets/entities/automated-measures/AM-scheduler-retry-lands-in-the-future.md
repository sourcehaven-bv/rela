---
id: AM-scheduler-retry-lands-in-the-future
type: automated-measure
title: A retry scheduled after a slow failure lands in the future, and a wedged task advances the ladder
description: Three tests in internal/scheduler/scheduler_test.go covering the retry clock and the skip path. Two of them fail against the pre-fix code with the exact production symptom; the third guards the healthy-slow-task behaviour the skip path exists for.
kind: test
location: internal/scheduler/scheduler_test.go
status: active
---

Regression tests for BUG-P3JGWR.

The defect needed a run whose **elapsed time exceeds the retry delay** to be
visible at all. Every pre-existing test used an effectively instantaneous run,
which makes `start` and `now` indistinguishable and hides the bug completely —
so the measure is as much about the fixture as the assertions.

## `TestRecordFailure_slowFailureRetriesInTheFuture`

Drives a failing task through the real `doExecuteTask` with a 20-minute run
duration against the 5-minute first rung, mirroring the `taskResultTimeout`
pairing that triggered this in production.

Asserts the retry lands strictly **after the failure that produced it**, and
pins the exact value at `failure time + one rung`. The first assertion is the
one that matters: a retry stamped in the past is due on the next tick and every
tick after, forever.

## `TestDoExecuteTask_wedgedSkipAdvancesLadder`

Holds a real in-flight claim so `enqueueTask` takes the genuine skip path — the
same path an orphaned queue row drives it down — with the retry ladder already
armed, which is the state a prior failure leaves behind.

Asserts the failure count advances and the new retry is in the future. Without
this the ladder stays pinned at one failure, never climbs through
10m/20m/40m, and `persistentFailureThreshold` never promotes the log line to
ERROR, so a permanently broken task is indistinguishable from a healthy one.

## `TestDoExecuteTask_healthySlowSkipLeavesStateAlone`

The guard on the other side. With **no** retry armed, a skip means a genuinely
slow healthy task, and must still record neither success nor failure.

This test is what keeps the second fix from over-reaching: without it, "advance
the ladder on a skip" could be widened to every skip, which would back off
healthy long-running tasks and suppress their normal cadence — the exact
behaviour the skip branch was written to protect.
