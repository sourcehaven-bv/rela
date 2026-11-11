---
id: TKT-7VAHS94
type: ticket
title: 'Flows: bound the cost of polling when: joins'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

The flow worker (TKT-FWQZGAM) tests a join that waits on a `when:` again every
`JoinRecheck` (5 minutes), for as long as the join waits. The code review of
TKT-FWQZGAM found three costs in that poll that do not block the worker but grow
with the number of waiting joins:

- Every waiting `when:` join costs a subject read, a guard evaluation and a run
write every 5 minutes, even when nothing changed. One worker handles about 5,000
runs per 5 minutes, so above that the 5-minute promise breaks and timers queue
behind the polls. A poll that did not fire counts as `Advanced`, so a busy
project logs "due runs" at Info on almost every tick.
- A rejoin that keeps failing (a store error on the join write) is tried again
every `minBackoff` (30s) with no growth; `branchesBackedOff` only backs off
branches.
- `put` calls `joinDue` on every write to a forked run, and `joinDue` resolves
the flow with `flowOf`, which may read and compile an older definition. Most
callers already hold the compiled flow.

## Acceptance

- A poll that does not fire is reported apart from a run that advanced, and
does not log at Info.
- The poll interval of a join backs off while its condition stays false, with a
cap, and a change event (TKT-GYXOTQC) resets it.
- A failing rejoin backs off with `backoffDelay`.
- `put` takes the compiled flow from callers that hold it.
