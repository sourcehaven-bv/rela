---
id: TKT-S5OJ7S
type: ticket
title: 'Session default_in: offer keyed types only where they apply'
kind: enhancement
priority: medium
effort: s
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

Add `session.default_in: keyed`, so a message type keyed by a transition may be
posted only in the states whose `on:` keys it, unless the type declares its own
`in:`. Without it, a config must copy `in:` from `on:` onto most of its types,
as the `pocRealtime` test fixture does.

It only fills `MessageType.In` at load, so it reuses the runtime of TKT-QYFLRH
unchanged. Follows TKT-QYFLRH. The key was first proposed as `session.offer:
transitions`; the design review renamed it (see PLAN-3JOPEX).
