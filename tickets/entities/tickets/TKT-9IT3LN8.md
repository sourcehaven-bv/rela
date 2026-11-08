---
id: TKT-9IT3LN8
type: ticket
title: 'Flows M1d: run flows end to end'
kind: enhancement
priority: medium
effort: l
status: backlog
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
  people can read. Decide whether the runtime refuses that when the created
  type's read audience is not within the subject's (G-004).

## Carried over from TKT-PV9R0GK review

The run store (`internal/flowrun`) is not wired yet; this ticket is its
first user.

- RR-U30RNUZ: record a pending-outputs mark in the state envelope
  (`flowrun.EncodeState`), so outputs of a move survive a crash between
  the store write and the side effects.
- RR-7E3BWVM: parse only the run's own flow from a snapshot. A snapshot
  that no longer compiles strands its run: report it, do not crash the
  timers loop.
- RR-DTSDBII: a start that races a subject delete or rename must not
  leave a run on a missing subject. `RekeySubject` and `List` by subject
  id work across faces.
- RR-R5I7UE0: the inbox reads `_flow-run` headers through the visibility
  gate, never the store.
- Expose runs to people only through the header's row gate: the store
  authorizes nothing.
- `rela db import-fs` does not copy runs yet. On fs they live in the
  `flow-runs.json` and `flow-definitions.json` state keys, which the
  import copies into `state_kv`, where the sqlite store does not read
  them. Decide whether the import moves them into `flow_runs` and
  `flow_definitions` or drops them, and update
  `TestDBImportFS_SQLiteTablesAreAccounted`.

## Carried over from TKT-BC5G8RP

- Call `metamodel.WithFlowRuns` from the wiring when flows.yaml declares
  flows. Until then nothing in production declares `_flow-run`.
- Write run headers only as `system:flow` (`principal.UserFlow`, tool
  `flow`), with config-derived values only (RR-K4KY2I8). Write outputs
  as `run_as`, and wrap step writes in `audit.WithFlowStep`.
- Queued jobs started inside a step do not carry the step (RR-LU55NIL).
