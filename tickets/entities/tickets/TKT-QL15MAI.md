---
id: TKT-QL15MAI
type: ticket
title: 'Flows: create output relating from the subject'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

A flow state's `create` output links the new entity to the subject with a
relation that must run from the new type to the subject
(`internal/flowcfg/states.go`, `relate`). Many schemas model "subject has X" the
other way round, as the tickets project does with `has-planning`,
`has-implementation`, `has-review` and `has-bug-analysis`. A trigger's `create`
already takes `relate.from: subject`; a flow output cannot, so it cannot create
those checklists (blocker R1 of DEC on TKT-XB4M29R).

Add `relate.from: subject` (and the default `from: new`) to flow `create`
outputs, with the load check, the `_produced` edge unchanged, and tests on every
backend.

## Acceptance

- A flow on the tickets schema can create a planning checklist linked by
`has-planning` from the ticket; `rela flow validate` accepts it.
- The reverse direction keeps working; a relation that links neither way is
still refused at load.
