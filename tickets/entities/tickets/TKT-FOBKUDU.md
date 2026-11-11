---
id: TKT-FOBKUDU
type: ticket
title: 'Direct messages 2/4: direct: block and managed _direct channel type'
kind: enhancement
priority: medium
effort: m
status: backlog
description: 'realtime.yaml direct: block compiling to a managed _direct channel type with a participant role source (DEC-42MOZY2).'
---

## Description

Add a `direct:` block to `realtime.yaml` that turns DMs on and compiles to a
managed `_direct` channel type: `attach_to: [_dm]`, `cardinality: one`, with a
new managed role source "participant" (a member of the anchor `_dm`). Design:
DEC-42MOZY2.

- `direct:` declares `message_types`, `typing` and `message_ops` with the
existing channel-type grammar. Not allowed: `lifecycle`, `session`, triggers,
`promote_to`, `create`, `read`, `roles`.
- Without person mapping (`user_entity_type` + `principal_property`) the
file fails to load with a clear message.
- Removing `direct:` hides DMs; channels follow `detached_retention`.
- `GET /api/v1/_schema/channels` describes the `_direct` type.

## Acceptance criteria

1. A `direct:` block with a `text` message type loads; both participants
post and read; a third person gets 404.
2. Each disallowed key is refused at load with a message naming it.
3. `direct:` without person mapping is refused at load.
4. Hot reload of realtime.yaml adds and removes DM support without restart.
