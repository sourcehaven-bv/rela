---
id: AM-scheduler-failed-task-no-retry-storm
type: automated-measure
title: A failing scheduled task does not re-execute on the next tick
description: 'Table-driven test across every schedule kind (day, weekday, week, interval): a task whose execution always errors must execute exactly once per its declared schedule, not once per tick.'
kind: test
location: internal/scheduler/ (test to be written with the fix)
status: proposed
---

Pins BUG-YZ13IJ.

A scheduled task whose execution always returns an error, advanced by one tick,
must execute exactly **once** — not once per tick.

The test must be **table-driven across every schedule kind** (`day`, a weekday,
`week`, an interval). That breadth is the point: the original report
hypothesised the bug was in `IsDue`'s `dayKind` comparison, which would have
limited it to `every: day`. Reproduction showed `runDueTasks` takes the
`!recorded` branch and **never calls `IsDue` at all**, so every schedule kind
amplifies to tick rate. A test covering only `day` would miss a regression that
reintroduces the short-circuit for other kinds.

Assert on execution *count* via the existing `executeTaskFunc` seam, with the
clock injected through `Scheduler.now`. Both seams already exist — no production
code needs to change to make this testable.

Must **fail on current `develop`**: today the task executes on every tick.

Companion assertions worth including in the same suite:

- A task that fails then succeeds resumes its normal cadence.
- A task that has never succeeded is still eventually retried (no permanent wedge).
- Consecutive failures back off rather than retrying at tick rate.
