---
id: TKT-771TAL2
type: ticket
title: 'Threads 5: document reply threads'
kind: docs
priority: medium
effort: s
status: backlog
---

## Description

Slice 5 of RES-TU7ZMMR (from TKT-UQ0BM5G). Document threads in `docs-project/`
(generated into `docs/realtime.md` and the data-entry guide).

### Scope

- Replies and threads: one level, inline exceptions, the panel.
- Read markers: channel and thread markers, which counts each covers.
- Followers: automatic follows, mute, the cap of 50.
- Inbox: the `thread` kind and its precedence.
- HTTP API, MCP tools and CLI flags added by slices 1 to 3.

### Acceptance criteria

- Every route, tool, flag and limit added by slices 1 to 4 is documented.
- The docs build passes.
