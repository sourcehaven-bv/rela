---
id: TKT-W5ZSYJ
type: ticket
title: Busy board live refresh on session moves
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

A board with a `busy:` card field (TKT-21G6JS) refreshes live only on graph
writes. A session move that writes no graph output shows on the next reload.
Wake busy list subscriptions on session moves of the busy channel type, selected
on the gated page anchors (not on every channel write), with the inbox's
no-fallback treatment and a recheck for syncing subscriptions. See the
TKT-21G6JS design review (live wake, fallback oracle, syncing gap).
