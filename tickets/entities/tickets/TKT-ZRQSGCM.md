---
id: TKT-ZRQSGCM
type: ticket
title: 'Flows M5c: delete the old transition and automation paths and dogfood the tickets workflow'
kind: refactor
priority: medium
effort: m
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

Third part of the M5 split of TKT-12RC7JT.

- Delete the old evaluation paths: the automation engine in
`internal/automation` and the transition enforcer in `internal/statemachine`,
once M5a (TKT-12RC7JT) and M5b run them on the flow engine. No second evaluation
path remains.
- For transitions, concretely: fold `compileMachine` and the enforcer of
`internal/statemachine` into `flowcfg`/the flow engine, show headless flows in
`rela flow` describe and list, and decide whether a headless flow reads an unset
property as its initial state (M5a keeps today's answer: no, the move is
refused).
- Dogfood: express the tickets checklist workflow (planning,
implementation, review and docs checklists created on status changes) as a flow
in `tickets/flows.yaml`, with a data migration where existing projects need one.

## Acceptance

- `internal/automation` and the statemachine enforcer have no callers left
and are removed.
- The tickets project runs its checklist workflow from `flows.yaml`, and
the three analyze gates stay clean.

## Outcome

The schema transitions now live in their own package,
`internal/flowcfg/transitions`, beside the flows.yaml loader in `flowcfg`. The
automation deletion and the tickets dogfood moved to TKT-GWOBH2U and
TKT-XB4M29R.
