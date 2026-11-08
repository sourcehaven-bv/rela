---
id: TKT-QROHRJ0
type: ticket
title: 'Flows: stable rule ids for flow validation errors'
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

Give every `rela flow validate` (and `flows.yaml` load) error a stable rule id,
such as `FLOW012`, and add a docs page that lists every rule: what it rejects,
why, and how to fix it. Inspired by Waymark, which names every unsupported
pattern and publishes its known gaps.

- Ids are stable once published; retired ids are not reused.
- The error message names the id, the flow and the step or state.
- The docs page lives in `docs-project/` (docs are generated from it).

Do it with or right after the runtime ticket TKT-9IT3LN8, when the set of load
checks is settled.

## Acceptance

- A test pins that every check emits a documented id.
