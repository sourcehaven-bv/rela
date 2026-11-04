---
id: TKT-LKIVLR2
type: ticket
title: Group quiet channel messages across session notices
kind: enhancement
priority: high
effort: m
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

## Description

Quiet message types (`display: collapsed` / `hidden` in the `channels:` block of
`data-entry.yaml`, TKT-03DMXH) fold a run of consecutive quiet messages into one
line. A session notice (the line the server posts when a message or timer moves
the session, for example "Planning -> Implementing") ends the run. An agent run
that posts progress, moves the session, posts more progress, and so on, still
fills the log with many small fold lines with a notice between each pair
(FINDINGS.md section 5, generic gap 6).

Let quiet messages and the session notices between them fold into one group.
Anything the reader must read or act on still ends the group and stays visible:
a kept row (the waiting message with its decisions or choices, a row with a type
action, a row the reader asked for), a message of a type that is not quiet, and
a notice that moves the session into a state that waits for someone or ends it.

Reuse the existing fold (`channelFold.ts`, `RlChannelFold`, `RlChannel`); do not
build a second one. Keyboard and screen-reader access stay: the fold line is a
button with `aria-expanded`, and a state move is still announced. Check the
interaction with the decision placement from TKT-A0RTC52, unread counts, the
inbox row (TKT-V05706X) and the placed channel block (TKT-VXBE16H). Prefer a
good default; add config under `channels:` only if needed.

Follows TKT-03DMXH, TKT-A0RTC52, TKT-V05706X and TKT-VXBE16H.

Plan: `.ignored/plan-TKT-LKIVLR2.md` (planning done, awaiting approval).
