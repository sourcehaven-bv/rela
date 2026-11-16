---
id: TKT-G33R2GW
type: ticket
title: 'Flows: idempotent create and conditional outputs'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Flow `create` outputs have no `if_exists`, and outputs have no condition. A run
that re-enters a state creates its outputs again, and a `set` cannot be limited
to "only when the property is empty". Triggers have both (`if_exists: skip`,
`where:`). A lifecycle flow whose states can be re-entered (review back to
in-progress) needs them (blocker R4 of the decision on TKT-XB4M29R).

Add `if_exists: skip` to flow `create` outputs (scoped to entities the run or
the subject already has through the same relation) and an optional `when:` on an
output, held to the flow condition rules.

## Acceptance

- Re-entering a state with `if_exists: skip` creates no second entity.
- An output with a false `when:` writes nothing and the run still takes its
`done` branch.
