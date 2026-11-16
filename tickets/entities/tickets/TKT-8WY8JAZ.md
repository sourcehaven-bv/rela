---
id: TKT-8WY8JAZ
type: ticket
title: 'Flows M5f: stateful ticket and bug flows in tickets/flows.yaml'
kind: refactor
priority: medium
effort: l
status: backlog
---

## Description

Express the ticket and bug lifecycle as stateful flows in `tickets/flows.yaml`
once the engine can carry them. The design sketch and the blockers are in
DEC-617HJHB.

- `ticket-work` and `bug-work`: start on create, states are the status values,
`change` branches driven by CLI edits, `waiting_for: [agent]`.
- A tickets `acl.yaml` with an `agent` role that keeps today's CLI behaviour.
- Checklists move into state outputs only when flow `create` can relate from
the subject and skip existing entities; until then they stay triggers.
- Carry over open tickets: start a run for every ticket and bug not in `done`
or `wont-fix`, in the state matching its status (a data migration or `rela flow
start` script).
- Extend `tickets/tests/checklist-workflow.test.lua` to cover the runs.

## Acceptance

- `rela update -P status=...` from the CLI moves the run.
- Every status change keeps creating the same checklist and relation.
- The three analyze gates stay clean.
