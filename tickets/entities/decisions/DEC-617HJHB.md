---
id: DEC-617HJHB
type: decision
title: Ticket lifecycle stays trigger-driven until flows can carry it
context: TKT-XB4M29R asked whether the ticket and bug lifecycle should become a stateful flow after M5d moved the checklist automations to flows.yaml triggers. A draft flow on a scratch copy of tickets/ showed that runs do not move on CLI edits, flow create outputs cannot use the has-* checklist relations, and run state on the file backend is not shared through git.
consequences: tickets/flows.yaml keeps its triggers and schema.yaml keeps the workflow validation rules. tickets/tests/checklist-workflow.test.lua proves every trigger in CI. Ticket and bug statuses get no transitions for now. The stateful flow is the M5f ticket, which waits on TKT-GYXOTQC and the engine follow-ups.
date: "2026-11-16"
status: accepted
---

## Context

M5d (#2350) moved the tickets checklist automations to `triggers:` in
`tickets/flows.yaml`. TKT-XB4M29R asked whether the ticket and bug lifecycle
should also become a stateful `flows:` entry. A draft flow was run on a scratch
copy of `tickets/` with the branch binary (PLAN-L879I8W).

Blockers on today's engine:

- R2 (lead). Runs do not start on create, ordinary edits emit no `change`
events, and the CLI has no command to do a step (TKT-GYXOTQC). Agents move
tickets with `rela update -P status=...`, so a run would never advance.
- R1. A flow `create` output must relate the new entity to the subject;
`has-planning`, `has-implementation`, `has-review` and `has-bug-analysis` run
from the subject to the checklist.
- R3. On the file backend the `_flow-run` entity is committed with the data
but its working state stays in the local `.rela/`. Agents work in many
worktrees, so other checkouts would see runs that cannot advance and that block
renaming their subject.
- R4. Flow `create` has no `if_exists` and outputs have no condition.
- R5. Status is free (no `transitions:`), so a flow only shadows `status`,
and `outside_edits:` does not apply to the CLI.
- R6. `waiting_for:` and `outside_edits:` need `acl.yaml`; `tickets/` has none.
- R7. A `rela` binary without `flows.yaml` support ignores the triggers and
creates no checklists; only a project version floor could prevent that.
- R8. A status trigger fires on a change, not on create.

## Decision

The ticket and bug lifecycle stays trigger-driven. `tickets/flows.yaml` keeps
its triggers, `schema.yaml` keeps the workflow validation rules, and
`tickets/tests/checklist-workflow.test.lua` proves every trigger in CI.

Ticket and bug statuses get no `transitions:` for now. They would be enforced on
CLI writes and refuse moves agents make today, such as backlog to done.

## Future flow (M5f sketch)

Tracked in TKT-8WY8JAZ. Engine follow-ups: TKT-QL15MAI (R1), TKT-G33R2GW (R4),
TKT-IAQO7HG (R3).

- Flow `ticket-work`: subject `ticket`, `per_subject: one`, started
`on: create`.
- States are the status values. Each non-final state has `change` branches
with `when: "entity.status == '<next>'"`, driven by change events from CLI edits
once TKT-GYXOTQC lands, so agents keep using `rela update`.
- `waiting_for: [agent]` needs a tickets `acl.yaml` with an `agent` role.
- `done` is final with status `completed`; `wont-fix` is final with status
`cancelled`.
- Checklists and dates stay triggers until R1 and R4 are fixed, then may move
into state outputs.
- `bug-work` has the same shape with `analyzing`.
- Run state needs R3 fixed, or the tickets project moves to a backend that
stores runs with the data.
