---
id: TKT-ZO9IB6V
type: ticket
title: 'Flows M4a: parallel branches'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Parallel branches (part B of the M4 split). Job steps moved to their own ticket,
which depends on this one and on the flow worker (TKT-FWQZGAM). The docs and CLI
fix for sourcehaven-bv/rela#2170 is its own ticket too.

- Fork and join: the run's single state becomes a set of active states (a
marking). A join is a predicate over the marking or the graph. Load checks
reject a fork without a matching join.

## Acceptance

- Existing single-state runs keep working unchanged, on every backend.
- Tests cover fork, join and single-state backward compatibility.
- The flowrun conformance tests pass on kv, sqlite and postgres.
