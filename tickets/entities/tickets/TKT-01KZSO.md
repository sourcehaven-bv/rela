---
id: TKT-01KZSO
type: ticket
title: OAuth token binding and Basecamp reference connector
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Credential handling and a reference integration, to prove the design of
FEAT-XYQMUB end to end.

1. **Writable token binding**: Lua scripts that declare it may store a rotated
OAuth refresh token. Stored encrypted in `state.KV` (shared across nodes on
postgres). Only one process refreshes a given connection at a time.
2. **Reference OAuth script**: a small script outside rela that runs the
one-time consent flow and writes the first token.
3. **Basecamp reference connector**: a Lua script that pushes through the
background automation action and pulls on a schedule or from a webhook. It
writes as a system principal (for example `system:basecamp`).

## Notes

- `secrets.yaml` stays operator-authored and read-only.
- Prefer machine credentials where a platform offers them. Otherwise document
the bot-user pattern.
