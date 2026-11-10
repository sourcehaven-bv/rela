---
id: TKT-FWQZGAM
type: ticket
title: 'Flows M1e: triggers, events, worker, retention and inbox'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Part 2 of TKT-9IT3LN8. Part 1 runs flows a person starts with `rela flow start`:
outputs as run_as, `done` branches, final states, cancel, the run store on every
backend, and `db import-fs`. This ticket finishes the runtime.

- Start triggers `create`, `predicate` and `schedule` (a basic schedule;
TKT-311Q282 and TKT-QROHRJ0 stay separate).
- `change` events through a write-path hook that knows the author, so a
branch's `roles:` is checked against the person who made the change. RR-EMIUGI8:
a run's own outputs (writes as run_as with the flow step on ctx) do not count as
`change` events.
- A flow worker in long-running hosts that calls `Engine.Due` under a
leader lock, so timers fire and pending outputs are recovered. Nothing calls
`Due` in a server yet.
- Deleting a subject cancels its runs; renaming it rekeys them
(`flowrun.Store.RekeySubject`). RR-DTSDBII: part 1 handles the start race;
delete and rename are here.
- Retention: archive or delete finished runs after `retention.after`.
- Waiting runs in `_inbox`, read through the header's visibility gate
(RR-R5I7UE0).
- RR-LU55NIL: queued jobs started inside a step carry the step.
- `db load` into an existing sqlite database (the in-transaction import)
refuses a source with unfinished runs, because `sqliteflowrun` needs its own
connection. Copy them there too.

## Acceptance

- The sample flow starts on create, moves on a change by hr, escalates on
its timer and finishes, on fs, postgres and sqlite.
