---
id: TKT-I1OLJ3V
type: ticket
title: 'Flows: start runs on create'
kind: enhancement
priority: high
effort: m
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

## Description

Split from TKT-GYXOTQC. Run the `on: create` start trigger of flows.yaml. When
an entity of a flow's subject type is created through the entity manager (API,
MCP, CLI create, Lua `rela.create_entity` in an action or webhook action, a
trigger's create output) and the optional `when:` holds, one run starts as the
flow's `run_as`.

- The header and the run are written in the create's store transaction on
sqlite and postgres, so a rolled-back create starts no run and a create starts
at most one run per flow.
- The run settles right after the commit; the flow worker retries a
failed settle.
- `per_subject: one` skips a subject that already carries a run.
- A chain of create starts is bounded (a flow whose output creates its
own subject type).

Not covered: `rela import`, `rela db load`, conversion, copy and history
restore. `on: predicate` and `on: schedule` stay on TKT-GYXOTQC.

## Acceptance

- The onboarding sample starts on the create of a hired person, settles
its first state, and starts nothing for a new person, on fs, sqlite and
postgres.
- A rolled-back batch leaves no run.
