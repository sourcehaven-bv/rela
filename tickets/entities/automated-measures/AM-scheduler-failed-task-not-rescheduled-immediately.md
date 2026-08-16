---
id: AM-scheduler-failed-task-not-rescheduled-immediately
type: automated-measure
title: A failed scheduled task is not re-run before its next scheduled slot
description: Table test driving a failing task through runDueTasks across several ticks, asserting it executes once per scheduled interval rather than once per tick, for both the no-prior-state and stale-state paths.
kind: test
location: internal/scheduler/scheduler_test.go (test to be written with the fix)
status: proposed
---

Regression test for BUG-ZKK2UL.

Drives a task whose execution always fails through repeated `runDueTasks` calls
with a controlled `s.now`, and asserts the execution count matches the
**schedule**, not the tick rate.

Two cases must both be covered, because they fail through different branches:

1. **No prior state** — the `!recorded` "first run" branch. With a `day`
schedule, ticking 10 times within the same day must yield exactly one execution,
not ten.
2. **Stale state after an earlier success** — the `IsDue(lastRun, now)`
branch. A task that succeeded yesterday and fails today must execute once today,
then not again until tomorrow.

A third assertion pins the secondary defect: the recorded timestamp is the run's
**start** time, so a task that starts at 23:59 and runs past midnight does not
skip the following day.
