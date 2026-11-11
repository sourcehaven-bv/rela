---
id: TKT-8YOZ51W
type: ticket
title: 'Direct messages 3/4: open a DM over HTTP, MCP and CLI'
kind: enhancement
priority: medium
effort: m
status: backlog
description: Idempotent open-DM call and DM listing on the HTTP API, MCP and CLI, with inbox integration (DEC-42MOZY2).
---

## Description

Add the call that opens (or finds) the DM between the caller and another person,
on every surface. Design: DEC-42MOZY2.

- Service: the caller must be a person (not an agent or system principal),
the target must be a user entity the caller can read, and not the caller. The
call is idempotent: the hashed pair id finds the existing `_dm`. Concurrent
opens of one pair create one `_dm`.
- HTTP: `POST /api/v1/_direct` with the target user id, answering the `_dm`
and its channel path; `GET /api/v1/_direct` lists the caller's DMs with unread
counts. OpenAPI updated.
- MCP tool and `rela channel dm` CLI command.
- DM channels appear in the inbox (`unread`, `mention`, `reply`) like other
channels.

## Acceptance criteria

1. Opening a DM twice returns the same `_dm`; two concurrent opens create
one.
2. A DM with yourself, with an agent, or with a person you cannot read is
refused; the last answers like a missing entity.
3. An agent caller is refused.
4. The listing shows only the caller's DMs, with unread counts.
5. The inbox lists unread DM messages for the recipient.
