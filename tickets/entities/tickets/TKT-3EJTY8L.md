---
id: TKT-3EJTY8L
type: ticket
title: 'Threads 2: per-thread read markers and unread counts'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Slice 2 of RES-TU7ZMMR (from TKT-UQ0BM5G). A reader gets one read marker per
thread, so opening a channel no longer marks thread replies read before they are
seen.

### Scope

- `thread_reads` per reader (user plus organization), channel and parent seq, holding a seq that only moves forward. Same privacy rules as channel markers: never visible to another reader, erased with the channel.
- The channel's unread count covers top-level messages and inline exceptions. A thread's unread count covers its replies above the thread marker.
- Marking the channel read also moves the thread marker of each inline exception's thread up to that reply.
- `PUT .../messages/<seq>/_read`; MCP `mark_read` gains `thread`; `rela channel mark-read --thread`.
- Reply and mention rows in a thread turn read against the thread marker.
- All backends and `channelstest`.

### Acceptance criteria

- Opening a channel with unread thread replies leaves them unread in the thread.
- A thread marker never moves back and sends a frame only to the reader's own subscriptions.
- A mention inside a thread leaves the inbox when the thread marker passes it.
- Renaming, detaching and purging a channel treat thread markers as channel markers are treated today.
