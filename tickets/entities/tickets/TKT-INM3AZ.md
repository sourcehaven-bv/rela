---
id: TKT-INM3AZ
type: ticket
title: 'Sessions 4/3: interrupting timers'
kind: enhancement
priority: medium
effort: m
started: "2026-10-22"
completed: "2026-10-24"
status: done
---

## Description

`after:` per state (from state entry or session start) fires a transition.
`next_due_at` on the channel row; the realtime worker leader runs a small timer
loop with a wake signal, recomputes on config reload, handles stranded states,
and freezes timers while the channel is closed (due timers are skipped and
restart from the reopen). Each fire is a CAS on (state, generation). Budget test
for the filechannels due scan.

Design: RES-O8I8S7 (decisions section). Part of FEAT-GWF8YH phase 4.
