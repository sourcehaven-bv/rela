---
id: TKT-ETL233O
type: ticket
title: 'Threads 1: read one thread and the top-level stream'
kind: enhancement
priority: medium
effort: m
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

## Description

Slice 1 of the reply threads design in RES-TU7ZMMR (from TKT-UQ0BM5G). Lets a
client read one thread, or the main stream without the replies it hides.

### Scope

- `GET .../messages?thread=<seq>`: the parent and its replies, with the existing `after`, `before`, `after_rev` and `limit` cursors.
- `GET .../messages?top=1`: top-level messages plus the inline exceptions. An inline exception is a reply of the answer type of a choice on its parent, or a reply that moved the session. The rule moves from `channelView.ts` into `internal/realtime`, so the server and the web app agree.
- `last_reply_at` on a parent, set in the same write that raises `reply_count`.
- All channel backends (file, sqlite, postgres) and `channelstest` conformance cases.
- MCP `read_channel` gains `thread` and `top`; `rela channel read` gains `--thread` and `--top`.

### Out of scope

Read markers, followers and the web app panel (later slices).

### Acceptance criteria

- A thread page holds the parent and only its replies, paged by seq and by `after_rev`.
- A `top=1` page never holds a reply that is not an inline exception, and a full page is never empty because of hidden replies.
- `last_reply_at` matches the newest reply on every backend; a removed reply does not change it.
- A thread on a parent the caller cannot read gets the same answer as the channel head.
