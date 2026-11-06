---
id: TKT-50DZEHI
type: ticket
title: 'Copies fire automations; fields: all with set: overrides'
kind: enhancement
priority: high
effort: m
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

Two gaps in copies, found while porting OpenVWR to rela
(`openvwr2/docs/04-platform-gaps.md` section 2 items 2 and 9, gap R11).

1. A copy fires no automations. `copyState` writes the target face, audits it
and sweeps files, but never calls the automation engine. Automations on the
target face (for example on `faces: [vastgesteld]`) that stamp a date or ping an
external system through a background action never run when a document is
established by a copy.
2. A copy cannot combine `fields: all` with a value to stamp. Stamping
`vastgesteld_op: "{{today}}"` while copying everything else needs a full
explicit field map.

## Proposal

1. After a copy commits, run the target face's `created` automations when the
copy created the face, and its `updated` automations otherwise. The automation
result and cascade run as on an ordinary create, through the gated mutator. A
new trigger key `on.copy: <name>` (or a list) limits an automation to writes
made by the named copies, and on its own fires on every run of the copy. The
loader refuses a name that `copies:` does not declare.
2. A new copy key `set:` maps target properties to values (the copy field
grammar, so `{{today}}` works) that a `fields: all` copy writes instead of the
source value. It needs `fields: all`, may not name a `keep:` property, a file
property or an external ref.

## Acceptance

- A copy that creates the target face runs `created` automations, and one that
overwrites it runs `updated` automations, including background actions.
- `on.copy:` narrows an automation to the named copies; an unknown copy name
is a load error.
- `fields: all` with `set:` stamps the set values and copies everything else.
- Docs in `docs-project/` describe both.
