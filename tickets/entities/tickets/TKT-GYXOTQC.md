---
id: TKT-GYXOTQC
type: ticket
title: 'Flows M1f: start triggers and change events'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Split from TKT-FWQZGAM, which now covers only the flow worker. This ticket adds
the start triggers and `change` events. It runs on the worker.

- Start triggers `create`, `predicate` and `schedule` (a basic schedule;
TKT-311Q282 and TKT-QROHRJ0 stay separate).
- `change` events through a write-path hook that knows the author, so a
branch's `roles:` is checked against the person who made the change.
- RR-EMIUGI8: a run's own outputs (writes as run_as with the flow step on
ctx) do not count as `change` events. A step completed in the run panel writes
with an `audit.FlowStep` on ctx and then calls `Change` itself, so the hook
skips writes that carry a flow step.
- `Engine.Change(ctx, id, gen, by, holds)` refuses a stale generation with
`ErrStale`. The hook passes the generation it read with the run.
- A `change` event tests a waiting join's `when:` at once. Until then the
worker tests it on a poll.
- RR-LU55NIL: queued jobs started inside a step carry the step.

## Acceptance

- The sample flow starts on create, moves on a change by hr, escalates on
its timer and finishes, on fs, postgres and sqlite.
