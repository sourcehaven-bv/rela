---
id: TKT-GYXOTQC
type: ticket
title: 'Flows M1f: predicate starts and change events from every write'
kind: enhancement
priority: medium
effort: m
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

## Description

Split from TKT-FWQZGAM, which now covers only the flow worker. This ticket adds
`predicate` start triggers and `change` events from ordinary writes. `on:
create` landed in TKT-I1OLJ3V. `on: schedule` moves to its own ticket.

Driver: an outside system's state synced into rela by a sync connector (Lua +
`rela scheduler`), such as a GitHub pull request becoming merged, should move a
flow on without a person acting.

- `change` events through a write-path hook that knows the author, so a
branch's `roles:` is checked against the principal that made the change.
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

1. **Every write is a change event.** An update through the data-entry API,
MCP, the CLI, a Lua script (action, webhook action, scheduled sync script) and
`rela import` takes the first `change` branch of the subject's open run whose
`roles` and `when` hold. `when` is evaluated on the subject as that write left
it. A write that changes no property value is not an event.
2. **`predicate` starts.** A run starts when a write makes the subject come to
match `when` (it did not match before the write, it does after). Subjects that
already match when the flow is deployed get no run. `per_subject: one` is
respected.
3. **Attribution.** The event's author is the writer's principal (for a sync,
its `run_as`, such as `integration:github`). `roles:` is checked against it and
the audit log records it.
4. **Once only.** The branch decision is stored in the write's transaction (on
postgres and sqlite), like a create-started run. A repeated identical sync write
moves a run once. A decision stored before a crash is settled by the worker
after the restart.
5. **Ordering.** Events are decided in commit order against the post-write
state of each write, never against a later state. A burst open then merged takes
the branches in that order.
6. **Loop guard.** A hop count carried through flow job writes bounds how far
a write can move or start other runs (see below).

Not in scope: a change on a related entity as an event for the subject. Use a
trigger on the related type whose script writes a property on the subject; that
write is an ordinary change event. Docs show the pattern.

- The sample flow starts on a predicate, moves on a change by hr and on a
change by a sync script, escalates on its timer and finishes, on fs, postgres
and sqlite.

## Loop guard for job steps (from TKT-PY2G11W)

Once a write can start a run, a flow job's writes can start another run whose
job writes again. Carry a hop count through the job handler (`appbuild` flow job
context), as `automationjobs.go` does with `jobHopsKey`, and refuse a start past
the bound.

## Follow-up

Issues #2606 and #2607: the flows guide said no trigger cascade write fires a change event, and suggested a roll-up as input for a predicate start. A Lua script action's `rela.update_entity` and a `set` on an update do fire the event; only a trigger's `set` written straight to the store does not. A roll-up is computed on read and never stored, so it starts nothing. The guide and the `UpdateHook` godoc now say so.

## Follow-up 2

Issues #2653 and #2654: the flows guide now says a background script's update is attributed to `run_as`. The ACL and data-entry guides now say a trigger's `create` and `relate` outputs are not checked against the caller's role grants; only the managed refusal applies.
