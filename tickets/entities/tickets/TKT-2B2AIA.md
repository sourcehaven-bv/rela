---
id: TKT-2B2AIA
type: ticket
title: 'Inbox 2: read markers and unread counts'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Store methods SetRead\, Reads and a grouped unread count in all channel backends
plus conformance; an own post moves the marker; unread in the channel list and
head; PUT .../_channels/{type}[/{name}]/_read; MCP mark_read; unread_channels in
the inbox; SPA counts in the channel panel. Counts only for channels the
principal opened or posted in\, capped at 100\, gated before counting.

Design: RES-B0MJAZ (slices table and decisions). Follows TKT-ZZ4PNX.
