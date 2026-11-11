---
id: TKT-FWQZGAM
type: ticket
title: 'Flows M1e: flow worker'
kind: enhancement
priority: medium
effort: l
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

## Description

Part 2 of TKT-9IT3LN8, narrowed to the flow worker. Part 1 runs flows a person
starts with `rela flow start`: outputs as run_as, `done` branches, final states,
cancel, the run store on every backend, and `db import-fs`. Nothing calls
`Engine.Due` in a server yet, so timers never fire and a step whose outputs
failed waits for a manual retry.

This ticket adds a flow worker to the long-running hosts (rela-server, each
resident tenant of the multi-tenant server, and rela-desktop). It calls
`Engine.Due` under a leader lock, so that:

- timers fire;
- a run left with a pending mark (a crash or a failed output) is settled
again, with the backoff the engine stores;
- a join whose `when:` became true after the last arrival continues: the
engine makes a partly arrived run with a `when:` join due again on a poll.

The leader lock is the realtime worker's: a keyed advisory lock on postgres, a
lock file on fs and sqlite, and a free lock in memory. The work per tick is
capped.

Also in scope: the review responses deferred to the worker from TKT-ZO9IB6V that
touch `Due` and the run state (RR-1D9R0UF, RR-ZRMWUYT), and the text TKT-U9MKKGD
(#2198) asked to restore once the worker lands: GUIDE-flows "Running flows",
GUIDE-mcp-server, `internal/cli/flow_runs.go`, `internal/mcp/tools_flows.go`,
`flowview.Completed.Problem`, and the `flows.waiting.pending` and
`flows.step.problem.settling` strings.

## Split

The rest of the original ticket moved to follow-ups that depend on this one:

- TKT-GYXOTQC: start triggers and `change` events (and the acceptance
test with the sample flow).
- TKT-8137DHO: cancel runs when their subject is deleted.
- TKT-H43D60E: retention of finished runs.
- TKT-IVL2LF9: waiting runs in `_inbox`.
- TKT-TWLN8UB: `db load` copies unfinished runs into sqlite.
- TKT-YNQPHPP: M2 and M4b review leftovers.
- TKT-ZOU5BCT: fire due runs of tenants that are not resident (from the
design review).

TKT-PY2G11W (job steps) depends on this ticket for backoff and timeouts.

## Acceptance

- On fs, sqlite and postgres: a timer escalates, a pending output is
recovered, and a join whose condition becomes true later continues on a later
tick, all without a person acting.
- Only one process runs `Due` at a time per project or tenant.
- One tick advances at most the per-tick cap of runs.
