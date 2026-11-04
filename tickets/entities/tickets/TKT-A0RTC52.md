---
id: TKT-A0RTC52
type: ticket
title: Show each session decision button once
kind: enhancement
priority: high
effort: m
started: "2026-11-04"
status: in-progress
---

## Description

When a channel session waits on the reader, the same decision buttons show in up
to three places on the entity page: the session header, the waiting message (the
message that moved the session into its current state), and the composer's type
picker, where each decision type is an option marked as moving the session. In
the agent runner POC, a plan review shows **Approve plan** and **Revise plan**
three times. That is noisy, and it is unclear where to act (FINDINGS.md section
5, generic gap 5).

Settle on one place per surface, with one rule for which place wins:

- the entity page channel (also the channel block placed above the
relations, TKT-VXBE16H, and the inbox drawer, which use the same component);
- the inbox row (TKT-V05706X), which must follow the same rule.

Reuse the existing components (`RlChannelDecisions`, `useSessionDecisions`,
`rowDecisions`); do not build a second implementation. Keyboard and
screen-reader access stay: the remaining buttons are reachable and announced,
and no session state becomes unreachable. Prefer a good default without
configuration; if configuration is needed, follow the `channels.<type>.session`
conventions in `data-entry.yaml` (TKT-56TPOTE).

Follows TKT-T9G1AE, TKT-03FQPD, TKT-V05706X, TKT-56TPOTE and TKT-VXBE16H.

Plan: `.ignored/plan-TKT-A0RTC52.md` (planning done, awaiting approval). A note
on a one-click decision is split out to TKT-HG4PO94.
