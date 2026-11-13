---
id: TKT-GS7THPA
type: ticket
title: 'Flows M5b: compile automations onto entity-change triggers'
kind: refactor
priority: medium
effort: l
started: "2026-11-13"
status: planning
---

## Description

Second part of the M5 split of TKT-12RC7JT. Compile schema `automations:` onto
the flow engine as entity-change triggers with outputs.

- An automation's trigger (created, updated, property change with
`becomes`/`from`, relation created or removed, deleted, `faces:`) and its
`when:`/`condition:` compile to a trigger the flow engine evaluates through
`internal/predicate`.
- Its actions compile to flow outputs. `set` and `create_entity` +
`create_relation` map onto the existing `set` and `create` + `relate` outputs.
The other actions (delete, notify, add_to_pile, script, `validate:`) need new
output kinds or stay as named exceptions; the planning phase decides which.
- Side effects (mail, HTTP, AI) stay on `jobs.Queue`.
- Every write still goes through `entitymanager`, keeps its audit record and
its `triggered_by` attribution, and keeps the cascade semantics of
`internal/autocascade`.

Its dependency on M5a (TKT-12RC7JT) is ordering only: it shares no code with the
transition work.

## Acceptance

- Every existing automation test in `internal/automation` and
`internal/autocascade` passes against the new path.
- The `tickets/` and `docs-project/` schemas load and their checklist
automations create the same entities and relations as before.
