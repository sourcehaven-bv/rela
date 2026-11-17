---
id: TKT-ZT2KREN
type: ticket
title: 'Configure: edit flows.yaml triggers'
kind: enhancement
priority: medium
effort: m
started: "2026-11-16"
completed: "2026-11-17"
status: done
---

## Description

TKT-GWOBH2U moves automations from `schema.yaml` to `triggers:` in `flows.yaml`
and removes the Configure automations editor, because Configure edits only
`schema.yaml` and `data-entry.yaml`.

Port the editor to `flows.yaml` triggers:

- Add `flows.yaml` as a Configure file, with a per-key allowlist for
`triggers.*` that matches the keys the old `automations.[]` allowlist opened.
Script, ACL-bypass, capability and background keys stay locked.
- Bring back the Automations view, routes and delete-reference warnings in the
SPA on top of the triggers model.
- The Configure save keeps its rules: `--config-editing` only, the
`config:edit` grant, the fs build only, and a `config-edit` audit record.

## Acceptance

- A `config:edit` holder can edit the same trigger keys in the browser as
before TKT-GWOBH2U, and locked keys stay read-only.
- A save that breaks a trigger is refused at preview.

## Notes

- CI Archplan: `CheckTriggerACL` takes `configedit.TriggerEdit`, the Snapshot
files are a named `configedit.Tree`, and problem paths use named flows.yaml
keys (RR-CNCGY15).
