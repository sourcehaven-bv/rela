---
id: TKT-IZ80KZX
type: ticket
title: Flow tasks as a next-action source
kind: enhancement
priority: medium
effort: m
started: "2026-11-18"
completed: "2026-11-19"
status: done
---

## Description

Waiting flow tasks can feed the next-action engine. A new built-in source kind,
`source: flow_tasks` under `next_actions:`, offers the reader's waiting flow
steps as suggestions, with an optional `flows:` filter and the usual `band`,
`visible_on`, `cooldown` and offers.

- Candidates are exactly the reader's tasks, computed by the same call as the
Flow tasks page (same read gate, global roles and role relations). No runtime
deny.
- The message uses `{flow}`, `{step}` and `{subject}` (the flow label, the
state label and the subject title), e.g. "Complete Company details for Northwind
Demo B.V.".
- Acting on it opens `/entity/<type>/<id>?run=<run>`; the Flows panel scrolls
to and focuses that run.
- Snooze and dismiss hide only the hint. A completed task drops out on the
next resolve. The source picks its oldest waiting task.

Use case: the harbourmaster customer portal shows a banner to the `details` step
of the `signup` flow.
