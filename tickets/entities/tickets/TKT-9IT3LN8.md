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
