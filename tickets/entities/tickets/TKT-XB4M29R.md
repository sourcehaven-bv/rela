---
id: TKT-XB4M29R
type: ticket
title: 'Flows M5e: dogfood the tickets checklist workflow in tickets/flows.yaml'
kind: refactor
priority: medium
effort: s
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Description

Third part of the M5c split of TKT-ZRQSGCM: dogfood the flows work on the
tickets checklist workflow.

M5d (TKT-GWOBH2U, #2350) already moved every checklist and date automation from
`tickets/schema.yaml` to the `triggers:` of `tickets/flows.yaml`, so the tickets
project runs its checklist workflow from `flows.yaml` today. This ticket proves
it end to end and records why the lifecycle does not become a stateful `flows:`
entry yet.

- Add a `rela test` suite, `tickets/tests/checklist-workflow.test.lua`, that
walks every ticket and bug status with a trigger, in a fresh in-memory copy of
the tickets config, and checks the checklist, relation and dates.
- A Go test fails when a trigger in `tickets/flows.yaml` has no case.
- The `rela-tickets` CI job runs `rela flow validate` and
`rela test --require-tests`.
- CLAUDE.md: build `rela` from develop at or after #2350; create tickets in
`backlog` or `ready` and then move the status.
- DEC-617HJHB records the blockers for a stateful ticket flow (PLAN-L879I8W,
R1 to R8), sketches the future flow, and records that ticket and bug statuses
get no `transitions:` for now.
- Follow-up tickets: TKT-QL15MAI (flow `create` relating from the subject),
TKT-G33R2GW (idempotent flow `create`), TKT-IAQO7HG (run state on git-shared
file projects), TKT-62NRDMJ (claude-workflow template wording) and TKT-8WY8JAZ
(M5f: the stateful ticket flow, depends on TKT-GYXOTQC and the engine
follow-ups).
- `rela test` gains `actor:related(ref, type)` so a case can check relations.

No data migration: no type, property or relation changes, and no runs.

## Acceptance

- `rela test --require-tests` passes in `tickets/` and covers every status
that has a trigger, for tickets, bugs and checklists; CI runs it with `rela flow
validate`.
- A Go test fails when a trigger in `tickets/flows.yaml` has no case.
- CLAUDE.md carries the binary-version and create-in-status notes.
- The decision and the follow-up tickets exist and are linked.
- The three analyze gates stay clean.
