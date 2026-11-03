---
id: TKT-Y6NJ6U
type: ticket
title: 'Sessions 4/1: session config and message-driven transitions'
kind: enhancement
priority: high
effort: l
started: "2026-10-21"
completed: "2026-10-22"
status: done
---

## Description

A channel type can declare a `session:` block: states, `initial`, `final`, and
`on:` transitions keyed by message type with `to`, `roles` and a `when:`
predicate. Load-time validation in realtimecfg. Session state (state,
generation, entered_at) on the channel row in every channel store backend, with
conformance tests. A transition is a CAS on (state, generation), posts a system
message and writes a channel audit record. The HTTP and MCP channel reads
include the session state. No timers, no graph outputs.

Design: RES-O8I8S7 (decisions section). Part of FEAT-GWF8YH phase 4.
