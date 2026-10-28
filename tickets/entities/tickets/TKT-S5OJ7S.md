---
id: TKT-S5OJ7S
type: ticket
title: Session option to offer transition types only where they apply
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Add `session.offer: transitions`, so a message type keyed by a transition may be
posted only in the states whose `on:` keys it, unless the type declares its own
`in:`. Without it, a config such as the agent-runner PoC copies `in:` from `on:`
onto most of its types.

It only fills `MessageType.In` at load, so it reuses the runtime of TKT-QYFLRH
unchanged. Follows TKT-QYFLRH.
