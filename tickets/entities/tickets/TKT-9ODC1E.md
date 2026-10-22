---
id: TKT-9ODC1E
type: ticket
title: Built-in typing indicator per channel type
kind: enhancement
priority: high
effort: m
started: "2026-10-22"
completed: "2026-10-22"
status: done
---

## Description

A typing indicator is today an operator-declared `ephemeral` message type (for
example `typing: { persist: ephemeral, ttl: 5s, rate: 1/s }`). The composer then
shows it as a tab next to the real message types, and a user has to post it by
hand. Nobody uses a typing indicator that way. Found while testing the atlas
chat demo (2026-10-21).

Typing should be a built-in concept that a channel type switches on or off, and
that then simply works.

## Design (agreed with the user)

- Channel type key `typing: true` (default off). Not a message type: it never
appears in the composer or the schema's message types.
- The server owns the signal: fixed ttl and rate (constants in
`realtimelimits`), kept like other ephemeral signals (process-local, not stored,
not audited).
- Whoever may post in the channel may send the typing signal; no separate role.
- The SPA sends it automatically while the user types (throttled to the rate)
and shows "<name> is typing…" for others until it expires. MCP may send it
optionally (agents).
- Operator-declared ephemeral types keep working for other signals.
- Future built-ins (read receipts, presence "viewing") will be sibling keys on
the channel type; introduce a grouping key only when a second one exists.

## Acceptance

- `typing: true` loads; an unknown value or `typing` on a type without any
postable message type is refused with a clear message.
- The composer never lists the typing signal; typing in the composer sends it,
throttled; another reader sees the indicator and it disappears after the ttl or
when the message is sent.
- HTTP, MCP and the schema endpoint expose whether typing is on.
- Docs: GUIDE-realtime.md.
