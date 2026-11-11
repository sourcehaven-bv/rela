---
id: TKT-9IT3LN8
type: ticket
title: 'Flows M1d: run flows end to end'
kind: enhancement
priority: medium
effort: l
started: "2026-11-07"
completed: "2026-11-08"
status: done
---

## Description

Runtime: start triggers (create, predicate, manual, schedule), per_subject,
events, timers, outputs as run_as (templates read through the run_as-gated
reader, RR-JIX6EZR), subject delete cancels the run, retention, rela flow start
| runs | cancel, waiting runs in _inbox.

## Acceptance

- The sample flow runs end to end on file, pg and sqlite.

Split from TKT-XHANLJU.

## Carried over from TKT-XHANLJU review

- RR-EMIUGI8: decide whether a run's own outputs count as `change` events
(a branch without `roles:` admits any write).
- RR-XC2WBH9: move the session engine callers onto the `flow` limits.
- RR-JIX6EZR: evaluate templates through the run_as-gated reader.
- RR-EIRWL5J: a `create` output can copy subject values into a type more
people can read. Decide whether the runtime refuses that when the created type's
read audience is not within the subject's (G-004).

## Carried over from TKT-PV9R0GK review

The run store (`internal/flowrun`) is not wired yet; this ticket is its first
user.

- RR-U30RNUZ: record a pending-outputs mark in the state envelope
(`flowrun.EncodeState`), so outputs of a move survive a crash between the store
write and the side effects.
- RR-7E3BWVM: parse only the run's own flow from a snapshot. A snapshot
that no longer compiles strands its run: report it, do not crash the timers
loop.
- RR-DTSDBII: a start that races a subject delete or rename must not
leave a run on a missing subject. `RekeySubject` and `List` by subject id work
across faces.
- RR-R5I7UE0: the inbox reads `_flow-run` headers through the visibility
gate, never the store.
- Expose runs to people only through the header's row gate: the store
authorizes nothing.
- `rela db import-fs` does not copy runs yet. On fs they live in the
`flow-runs.json` and `flow-definitions.json` state keys, which the import copies
into `state_kv`, where the sqlite store does not read them. Decide whether the
import moves them into `flow_runs` and `flow_definitions` or drops them, and
update `TestDBImportFS_SQLiteTablesAreAccounted`.

## Carried over from TKT-BC5G8RP

- Call `metamodel.WithFlowRuns` from the wiring when flows.yaml declares
flows. Until then nothing in production declares `_flow-run`.
- Write run headers only as `system:flow` (`principal.UserFlow`, tool
`flow`), with config-derived values only (RR-K4KY2I8). Write outputs as
`run_as`, and wrap step writes in `audit.WithFlowStep`.
- Queued jobs started inside a step do not carry the step (RR-LU55NIL).

## Split

The ticket is split. This PR is part 1: the engine, manual start, outputs,
`done` branches, final states, cancel, timers and pending recovery through
`Engine.Due`, the wiring on every backend, `rela flow start|runs|cancel` and `db
import-fs`. Part 2 is TKT-FWQZGAM: create, predicate and schedule triggers,
`change` events, the flow worker that calls `Due`, subject delete and rename,
retention, `_inbox` and RR-LU55NIL.

## Decisions on the carried-over findings

- RR-EMIUGI8: decided. A run's own outputs do not count as `change`
events. Part 1 has no `change` events; TKT-FWQZGAM implements the rule in the
write-path hook.
- RR-XC2WBH9: fixed. The session caps are gone from `realtimelimits`;
realtimecfg and realtime use the `flow` limits.
- RR-JIX6EZR: fixed. Templates read the subject as run_as through the
gated reader; a subject run_as cannot read gives empty placeholders.
- RR-EIRWL5J: decided. No runtime refusal. Comparing read audiences is
not decidable in general (role relations, `when:`), and a refusal would be a
runtime deny. It stays an author choice, documented in the flows guide, as for
automations that create entities.
- RR-U30RNUZ: fixed. `flowrun.State` carries a pending mark stored with
the move. It stays until the run waits or ends, so Due finishes outputs, `done`
branches and the end of the run after a crash or failure, with backoff. A
`create` is claimed before it is written; a failed create returns the claim, and
one whose entity was written counts as done, so it is never written twice.
- RR-7E3BWVM: fixed. A run on an older version compiles only its own
stored definition (`flowcfg.ParseFlow`). One that does not compile is
`ErrStranded`: Due logs and skips it, and it can still be cancelled.
- RR-DTSDBII: partly fixed. The header and `_carries` are written in one
transaction that first checks the subject exists, and Start re-checks after the
run is stored and cancels when the subject is gone. Delete and rename handling
move to TKT-FWQZGAM.
- RR-R5I7UE0: fixed for `rela flow runs`, which lists headers through the
visibility gate. The inbox follows in TKT-FWQZGAM.
- Runs reach people only through the header's row gate: fixed;
`appbuild.FlowControl` reads subjects and headers through the gated reader.
- `db import-fs`: decided and fixed. The import reads the source with
`_flow-run` added when flows.yaml declares flows, so headers and edges are
copied, and copies each unfinished run with its definition into `flow_runs` and
`flow_definitions`. A target without a run store (the in-transaction `db load`)
refuses a source with unfinished runs.
- `metamodel.WithFlowRuns` from the wiring: fixed, before the policy is
validated, so the ACL injects the engine role.
- RR-K4KY2I8: fixed. Headers are written as system:flow (tool `flow`)
with config-derived values only; outputs as run_as inside `audit.WithFlowStep`.
- RR-LU55NIL: deferred to TKT-FWQZGAM.
