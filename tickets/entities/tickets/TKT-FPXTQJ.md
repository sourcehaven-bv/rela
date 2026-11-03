---
id: TKT-FPXTQJ
type: ticket
title: Guarded branching transitions per message type
kind: enhancement
priority: medium
effort: m
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

## Description

`on:` is keyed by message type, so one type has one target per state. A "plan
decision: approve | revise" type with an enum and guarded transitions to
different states cannot be declared.

Allow a list of guarded entries per type; the first `when` that holds wins. This
also makes enum plus `select` forms useful.

Reported by the agent-runner PoC (rela-agent-runner-poc/rela-mission),
2026-10-28.
