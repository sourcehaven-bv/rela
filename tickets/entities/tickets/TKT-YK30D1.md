---
id: TKT-YK30D1
type: ticket
title: 'Inbox 6: cross-process live delivery'
kind: enhancement
priority: low
effort: m
status: backlog
---

## Description

A pg_notify poke for channel changes into realtime.Local, shared with channel
live frames. Part of realtime phase 5 (multi-instance).

Design: RES-B0MJAZ (slices table and decisions). Follows TKT-ZZ4PNX.

## Note from TKT-W5ZSYJ

Feed the poke's keys into rela-server's `realtime.Local` through `Local.Mark`,
which records them as moved keys, not only as durable ones. A poke cannot tell a
message append from a session or lifecycle move; with the message-only kind,
busy kanban boards would miss cross-process moves until the next safety turn.
With the moved kind, the TKT-W5ZSYJ busy wake refreshes an open board within one
500 ms tick for moves made by another process or replica on postgres. Other
backends need nothing here: TKT-W5ZSYJ adds busy boards to the inbox safety
replay, which bounds cross-process staleness to about 12 s on files and 14 s on
a database.
