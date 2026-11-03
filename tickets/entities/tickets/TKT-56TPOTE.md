---
id: TKT-56TPOTE
type: ticket
title: Compact session header by configuration
kind: enhancement
priority: high
effort: m
started: "2026-11-03"
status: review
---

## Description

A channel session header shows every state of the session as a pill in one
wrapping row. The agent runner PoC has 13 states, so the header takes two rows
and the current state is hard to find (FINDINGS.md, UX finding 2, gap 3).

A compact mode exists: one summary line and a **Show all states** toggle. It
only switches on when the channel is narrower than 32rem
(`RlChannelSession.vue`, the `@container (max-width: 32rem)` block). No
configuration can turn it on.

Add a configuration option on a channel type's session that turns the compact
mode on at every width. In that mode the header shows the current state
prominently, with the state the session came from and the states it can move to
next. The full list stays behind the existing toggle. Without the option the
header behaves as today: full list, folded below 32rem.

Reuse the existing compact mode; do not build a second one. The inbox row
(TKT-V05706X) shows only the state label as text, so it needs no change.

## Acceptance

- A channel type can opt in to the compact header by configuration.
- The compact header shows previous, current and next states; the toggle
shows the full list.
- Without the option, nothing changes.
- The option is checked at load, by `rela validate` and by Configure, and is
documented.
