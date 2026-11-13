---
id: TKT-12RC7JT
type: ticket
title: 'Flows M5a: compile enum transitions to headless flows'
kind: refactor
priority: medium
effort: l
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

## Description

Compile existing features onto the flow engine and remove the duplicate code.

- Enum `transitions:` compile to headless flows: the state is the entity's
own property, with no `_flow-run` header.
- `automations:` compile to entity-change triggers with outputs.
- Delete the old evaluation paths in `internal/automation` and the
transition code.
- Dogfood: express the tickets checklist workflow as a flow.

## Split

The whole of M5 does not fit one reviewable PR: it rewrites the hot write path
for transitions and automations at once. It is split into three sequential
tickets.

- M5a (this ticket): enum `transitions:` compile to headless flows. Each
state-machine custom type compiles to a `flow.Definition` (its enum values are
the states, its entry value is the initial state) plus its declared transitions
as branches. Enforcement on write and the performable-transition read decide
through `flow.Decide`, the decider realtime sessions and flowengine use. The
state is the entity's own property; there is no `_flow-run` header and no run
store entry. The old edge-map evaluation path in `internal/statemachine` is
removed.
- M5b (TKT-GS7THPA): `automations:` compile to entity-change triggers with
outputs.
- M5c (TKT-ZRQSGCM): delete the remaining old paths and dogfood the tickets
checklist workflow as a flow.

## Acceptance

- Existing schemas load and behave the same; their tests pass.
- Transitions have no evaluation path besides `flow.Decide`.
