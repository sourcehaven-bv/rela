---
id: TKT-OAX3DQ
type: ticket
title: Reload schedules and subscriptions without a restart
kind: enhancement
priority: medium
effort: s
started: "2026-10-14"
completed: "2026-10-14"
status: done
---

## Description

`schedules.yaml` and `subscriptions.yaml` are read once when the scheduler
starts. An operator who adds a feed or changes a schedule has to restart `rela
scheduler` or `rela-server` before the change takes effect (deferred from
TKT-RUO1GY).

## Approach

- The scheduler holds its config in an `atomic.Pointer[Config]`. Each operation loads it once.
- `Scheduler.WatchConfig()` makes every tick (60 s) reload both files through `LoadWorkspaceConfig`. A changed, valid config replaces the current one. `StartBackground` and `rela scheduler` call it.
- An invalid file keeps the running config, so a typo does not stop the feeds and tasks that work. The error is logged once per distinct message, and once more when it is fixed.
- `StartBackground` keeps running with no tasks, so a file added later is picked up.

## Acceptance criteria

1. Adding a subscription to a running scheduler makes it poll within one tick, without a restart.
2. Removing a task stops new runs of it; a run already queued finishes.
3. An invalid edit keeps the previous tasks running, and logs the error once.
4. A server started with no tasks picks up a `schedules.yaml` created later.
