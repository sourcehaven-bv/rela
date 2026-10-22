---
id: TKT-36Z6X3
type: ticket
title: 'Sessions 4/4: non-interrupting timers'
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Session-level `timers:` with `in: [states]` and `every:` post a typed system
message (reminders) without leaving the state, with a reminder counter usable in
guards.

Design: RES-O8I8S7 (decisions section). Part of FEAT-GWF8YH phase 4.
