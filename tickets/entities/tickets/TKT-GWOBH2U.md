---
id: TKT-GWOBH2U
type: ticket
title: 'Flows M5d: entity-change triggers in flows.yaml and delete internal/automation'
kind: refactor
priority: medium
effort: l
status: backlog
---

## Description

Second part of the M5c split of TKT-ZRQSGCM. Move automations out of
`schema.yaml` into `flows.yaml` and delete `internal/automation`.

- `flows.yaml` gains entity-change triggers: created, updated, property
change with `becomes`/`from`, relation created or removed, deleted, renamed,
`faces:` and `copy:`, each with `when:` and `condition:`. They compile in
`flowcfg` and fire through `flow.Fire`, as M5b does today.
- Their outputs use the flowcfg output vocabulary. `set`, `create` and
`relate` gain what automations need: templates over the new and old entity and
the acting user, the relation direction of `create_entity.relation`, and
`if_exists`.
- The named exceptions of M5b get a home as output kinds, not dropped:
`add_to_pile`, `notify`, scripts (inline, file and `background:` jobs with their
`capabilities:` and `allow_acl_bypass`) and `validate:` checks.
- A schema migration moves `automations:` from `schema.yaml` into
`flows.yaml` for existing projects.
- `internal/automation` is deleted; `internal/autocascade` executes the
fired outputs. Audit, `triggered_by`, cascade semantics and side effects on
`jobs.Queue` do not change.

## Acceptance

- `internal/automation` has no callers and is removed.
- The M5b goldens (tickets, docs-project, basecamp, dataentry fixture,
synthetic) pass against the new path after the migration.
- The migration converts every in-repo project and is idempotent.
