---
id: TKT-L4E6JWC
type: ticket
title: Roll-ups along a chain, over ordered siblings and in list conditions
kind: enhancement
priority: high
effort: xl
started: "2026-11-28"
completed: "2026-11-28"
status: done
---

## Description

Roll-ups for an OmniFocus-style task manager (found by the omnifocus probe):

- `along:` walks a to-one relation chain up to 32 levels and folds `exists`, `min` or `max` over the nodes, so a deferred or on-hold ancestor (project, group, folder, parent tag) can hold an action.
- `siblings_before` folds over the earlier siblings of an ordered, identity-scoped relation, for sequential blocking that stays right after a drag reorder.
- One level of roll-up over roll-up (for example `project.stalled` over `has_available_action`). Cycles and deeper nesting fail the load.
- A list `condition:` may read roll-ups. Postgres answers it with a per-candidate LATERAL leaf in the keyset scan; other backends use the Go path. A `visible:` `when:` that reads a roll-up fails the policy load.
- A list publishes `refresh_types`, so the SPA refetches it when a type its roll-ups read changes.
- `examples/omnifocus` uses these roll-ups instead of Lua materialization.

Threat model and security requirements:

- Every node, sibling and inner roll-up value is gated per reader before the fold, on every backend. A roll-up value property must be visible to the reader.
- The row cap is per source and counts gated rows only, so hidden rows do not change an answer. Each source also has a raw read ceiling per step (HopCap), and Postgres a 5 s statement timeout; both fail closed for that source.
- An absent value (over cap, timeout, withheld) fails closed under every comparison, including `not`.
- Config names reach SQL only as bind parameters.
- Accepted residual: hop and edge reads are not gated, so hidden volume can affect latency and the one-bit ceiling outcome. Both fail closed.

Structure: the new store helpers are package functions or methods of `boundedDB`, not `Store` methods, and the siblings options sit in `metamodel.SiblingsDef`, so the types stay within their plimsoll load lines. The four new archplan findings are accepted in the baseline.
