---
id: TKT-XB4M29R
type: ticket
title: 'Flows M5e: dogfood the tickets checklist workflow in tickets/flows.yaml'
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

Third part of the M5c split of TKT-ZRQSGCM. Express the tickets checklist
workflow as a flow in `tickets/flows.yaml`.

- The workflow creates the planning, implementation, review and docs
checklists, and the bug-analysis checklist, on status changes, and records the
`started` and `completed` dates. Today these are the `automations:` of
`tickets/schema.yaml`.
- Write it as one flow per subject type (ticket, bug) in `tickets/flows.yaml`
on the trigger vocabulary of the previous ticket, and remove the automations
from `tickets/schema.yaml`.
- Add a data migration if existing tickets need one.
- Every status transition keeps creating the same checklist and relation,
tested end to end on a scratch copy of `tickets/`.

## Acceptance

- The tickets project runs its checklist workflow from `flows.yaml`.
- The three analyze gates stay clean.
