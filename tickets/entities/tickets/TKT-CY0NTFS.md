---
id: TKT-CY0NTFS
type: ticket
title: Trigger placeholder for the current user
kind: enhancement
priority: medium
effort: s
started: "2026-11-22"
completed: "2026-11-22"
status: done
---

## Description

Found while configuring the harbourmaster customer portal. A note should record
its author: the person who created it in data-entry. Today that takes a Lua
trigger script that reads `rela.principal.user` and calls `create_relation`
(TKT-5U6NRR, docs/lua-scripting.md). That works, but it is a script for what is
one relation, and it needs `allow_acl_bypass` or a capability review like any
script.

A trigger's `relate` output already writes a relation from the new entity to an
interpolated id, and skips it when the id is empty. What is missing is a
placeholder for the acting user.

## Proposal

Add the trigger placeholder `{{current_user.id}}`. Its value is the user entity
the request's principal resolved to: on data-entry with `principal_property`,
the user entity id. Without a resolved user entity (the CLI, MCP without a
lookup, background jobs) it is empty, so a `relate` is skipped rather than
pointing at a raw login name.

```yaml
triggers:
  note-author:
    on: { created: true, entity: [note] }
    outputs:
      - relate: { type: authored-by, to: "{{current_user.id}}" }
```

The relation is written by the existing cascade relation path, so the metamodel
and the write path rules apply unchanged. The placeholder is not interpolated
into inline Lua code; scripts keep `rela.principal`.

## Acceptance Criteria

1. `{{current_user.id}}` loads in every interpolated field (set, relate.to,
create.*, add_to_pile.*, check.message). Test: parse tests.
2. With a principal whose user was resolved to an entity, the placeholder
renders that entity id; a `relate` with it creates the relation. Test: engine
tests.
3. Without a resolved user (no principal, raw principal, `unknown`), it renders
empty and the `relate` is skipped. Test: engine tests.
4. The value comes from the principal, the identity the audit records; a
stamped query identity is not read. Test: engine tests.
5. Inline Lua code does not get it. Test: template tests.
6. The flows guide documents the placeholder and the note-author example; the
Lua route stays documented.
